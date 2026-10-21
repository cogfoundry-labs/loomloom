package workflowpreview

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalHTTPBoundaries(t *testing.T) {
	current := &state{snapshot: Snapshot{Protocol: Protocol, Sequence: "1"}}
	h := handler("127.0.0.1:1234", "secret", testViewer(t), current)
	for _, tc := range []struct {
		path, host, origin, token, method string
		status                            int
	}{
		{"/api/snapshot", "127.0.0.1:1234", "", "secret", "GET", 200},
		{"/api/snapshot", "127.0.0.1:1234", "", "", "GET", 401},
		{"/api/snapshot", "evil.test", "", "secret", "GET", 403},
		{"/api/snapshot", "127.0.0.1:1234", "https://evil.test", "secret", "GET", 403},
		{"/api/snapshot", "127.0.0.1:1234", "null", "secret", "GET", 403},
		{"/api/snapshot", "127.0.0.1:1234", "", "secret", "POST", 405},
		{"/", "127.0.0.1:1234", "", "", "GET", 200},
		{"/assets/", "127.0.0.1:1234", "", "", "GET", 404},
		{"/../secret", "127.0.0.1:1234", "", "", "GET", 404},
		{"/etc/passwd", "127.0.0.1:1234", "", "", "GET", 404},
	} {
		t.Run(tc.path+tc.host+tc.origin+tc.token+tc.method, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://127.0.0.1:1234"+tc.path, nil)
			r.Host = tc.host
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Authorization", "Bearer "+tc.token)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d want %d", w.Code, tc.status)
			}
			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("unsafe cache/CORS headers")
			}
		})
	}
	r := httptest.NewRequest("GET", "http://127.0.0.1:1234/api/snapshot", nil)
	r.Header.Set("Authorization", "Bearer secret")
	r.Header.Set("If-None-Match", "\"0\"")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 304 {
		t.Fatal(w.Code)
	}
}
func TestExternalLinkCanNavigateButCannotReadDraft(t *testing.T) {
	h := handler("127.0.0.1:1234", "secret", testViewer(t), &state{})
	for _, path := range []string{"/", "/api/snapshot"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:1234"+path, nil)
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		r.Header.Set("Sec-Fetch-Mode", "navigate")
		r.Header.Set("Sec-Fetch-Dest", "document")
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		want := 200
		if path != "/" {
			want = 403
		}
		if w.Code != want {
			t.Fatalf("%s got %d", path, w.Code)
		}
	}
}

func TestRunLocalDraftUpdatesWithoutRemoteRequests(t *testing.T) {
	path := filepath.Join(t.TempDir(), "draft.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan Event, 20)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{File: path, ViewerURL: testViewer(t).String()}, func(e Event) error { events <- e; return nil })
	}()
	next := func(kind string) Event {
		t.Helper()
		for {
			select {
			case e := <-events:
				if e.Type == kind {
					return e
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("missing %s", kind)
			}
		}
	}
	ready := next("ready")
	u, err := url.Parse(ready.ViewURL)
	if err != nil || u.Hostname() != "127.0.0.1" {
		t.Fatal(ready)
	}
	token, _ := url.ParseQuery(u.Fragment)
	u.Fragment = ""
	u.Path = "/api/snapshot"
	fetch := func() Snapshot {
		t.Helper()
		r, _ := http.NewRequest("GET", u.String(), nil)
		r.Header.Set("Authorization", "Bearer "+token.Get("token"))
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var snap Snapshot
		if err = json.NewDecoder(resp.Body).Decode(&snap); err != nil {
			t.Fatal(err)
		}
		return snap
	}
	if fetch().Content != nil {
		t.Fatal("missing file exposed content")
	}
	raw := `{"steps":[{"stepId":"s1","executionBinding":{"kind":"capabilityProfile","profileId":"text"},"inputBindings":{"prompt":{"source":"literal","value":"PRIVATE PROMPT"}}}],"workbook":{"secret":"PRIVATE WORKBOOK"}}`
	if err = os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	next("synced")
	snap := fetch()
	if snap.Content == nil || *snap.Content != raw || snap.Sequence != "1" {
		t.Fatal("bad local snapshot")
	}
	if err = os.WriteFile(path, []byte(`{"steps":[`), 0600); err != nil {
		t.Fatal(err)
	}
	next("synced")
	if *fetch().Content != `{"steps":[` {
		t.Fatal("incomplete JSON not preserved")
	}
	if fetch().Problem != "invalid_json" {
		t.Fatal("missing structured JSON problem")
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	next("waiting")
	if err = os.WriteFile(path, []byte(`{"steps":[`), 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for fetch().Problem != "invalid_json" {
		if time.Now().After(deadline) {
			t.Fatal("restored invalid file lost JSON problem")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown blocked")
	}
	resp, err := http.Get(u.String())
	if err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		t.Fatal("server remains reachable after exit")
	}
}
func TestReadFileRejectsUnsafeInput(t *testing.T) {
	dir := t.TempDir()
	if _, err := readFile(dir); err == nil {
		t.Fatal("accepted directory")
	}
	path := filepath.Join(dir, "draft")
	for _, body := range [][]byte{[]byte{0xff}, []byte(strings.Repeat("x", MaxContentBytes+1))} {
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readFile(path); err == nil {
			t.Fatal("accepted invalid content")
		}
	}
}

func testViewer(t *testing.T) *url.URL {
	t.Helper()
	u, err := validateViewerURL("https://viewer.example/workbench/viewer/v1/bootstrap.js")
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestBootstrapContainsOnlyPublicEntry(t *testing.T) {
	content := "private draft text"
	h := handler("127.0.0.1:1234", "private-session-token", testViewer(t), &state{snapshot: Snapshot{Content: &content}})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:1234/", nil))
	body := w.Body.String()
	if strings.Contains(body, content) || strings.Contains(body, "private-session-token") || !strings.Contains(body, "/viewer/v1/bootstrap.js") || !strings.Contains(body, "anonymous") {
		t.Fatal("unsafe startup page")
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "connect-src 'self'") {
		t.Fatal("snapshot connections must stay local")
	}
	for _, raw := range []string{"", "http://viewer.example/viewer/v1/bootstrap.js", "https://a:secret@viewer.example/viewer/v1/bootstrap.js", "https://viewer.example/viewer/v2/bootstrap.js", "https://viewer.example/viewer/v1/bootstrap.js?token=secret", "https://viewer.example/viewer/v1/bootstrap.js#secret"} {
		if _, err := validateViewerURL(raw); err == nil {
			t.Fatal("accepted invalid entry")
		}
	}
}
