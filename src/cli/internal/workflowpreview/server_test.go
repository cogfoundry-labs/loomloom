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
	"testing/fstest"
	"time"
)

func TestLocalHTTPBoundaries(t *testing.T) {
	current := &state{snapshot: Snapshot{Protocol: Protocol, Sequence: "1"}}
	h := handler("127.0.0.1:1234", "secret", fstest.MapFS{"viewer.html": {Data: []byte("viewer")}, "assets/a.js": {Data: []byte("asset")}}, current)
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
	h := handler("127.0.0.1:1234", "secret", fstest.MapFS{"viewer.html": {Data: []byte("viewer")}}, &state{})
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

func TestRunLocalDraftUpdatesAndNeverSendsContentToResolver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "draft.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan Event, 20)
	done := make(chan error, 1)
	resolved := make(chan Bindings, 10)
	resolve := func(ctx context.Context, b Bindings) (json.RawMessage, error) {
		resolved <- b
		return json.RawMessage(`{"schemaVersion":"workflow-viewer/v1","source":"preview","state":"resolved","steps":[]}`), nil
	}
	go func() {
		done <- Run(ctx, fstest.MapFS{"viewer.html": {Data: []byte("viewer")}}, resolve, Options{File: path}, func(e Event) error { events <- e; return nil })
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
	b := <-resolved
	encoded, _ := json.Marshal(b)
	if strings.Contains(string(encoded), "PRIVATE") || strings.Contains(string(encoded), "inputBindings") {
		t.Fatal("draft escaped metadata whitelist")
	}
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
	select {
	case <-resolved:
		t.Fatal("incomplete JSON triggered resolution")
	default:
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

func TestLateContractResultCannotOverwriteNewDraft(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	current := &state{snapshot: Snapshot{Sequence: "1"}}
	jobs := make(chan resolveJob, 1)
	started := make(chan string, 2)
	release := make(chan struct{}, 2)
	done := make(chan struct{})
	resolver := func(ctx context.Context, b Bindings) (json.RawMessage, error) {
		id := b.Steps[0].StepID
		started <- id
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-release:
		}
		return json.Marshal(map[string]string{"stepId": id})
	}
	go func() { resolveLoop(ctx, resolver, jobs, current); close(done) }()
	jobs <- resolveJob{sequence: "1", bindings: Bindings{Steps: []Step{{StepID: "old"}}}}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("resolver did not start")
	}
	current.change(func(s *Snapshot) { s.Sequence = "2" })
	jobs <- resolveJob{sequence: "2", bindings: Bindings{Steps: []Step{{StepID: "new"}}}, key: [32]byte{1}}
	release <- struct{}{}
	select {
	case id := <-started:
		if id != "new" {
			t.Fatal(id)
		}
	case <-time.After(time.Second):
		t.Fatal("new metadata was not resolved")
	}
	current.RLock()
	value := string(current.snapshot.Context)
	current.RUnlock()
	if value != "" {
		t.Fatal("stale metadata became visible", value)
	}
	release <- struct{}{}
	deadline := time.Now().Add(time.Second)
	for {
		current.RLock()
		value = string(current.snapshot.Context)
		current.RUnlock()
		if strings.Contains(value, "new") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("new metadata not applied")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("resolver did not stop")
	}
}

// No edit should be necessary to recover from an authority outage represented
// as partial metadata rather than an HTTP error.
func TestResolverRetriesTransientPartialContextWithoutEditing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	current := &state{snapshot: Snapshot{Sequence: "1"}}
	jobs := make(chan resolveJob, 1)
	done := make(chan struct{})
	calls := make(chan int, 2)
	count := 0
	partial := json.RawMessage(`{"state":"partial","steps":[{"stepId":"one","status":"unresolved","reason":"dependency_unavailable"}]}`)
	resolved := json.RawMessage(`{"state":"resolved","steps":[{"stepId":"one","status":"resolved"}]}`)
	go func() {
		defer close(done)
		resolveLoop(ctx, func(context.Context, Bindings) (json.RawMessage, error) {
			count++
			calls <- count
			if count == 1 {
				return partial, nil
			}
			return resolved, nil
		}, jobs, current)
	}()
	defer func() { cancel(); <-done }()
	jobs <- resolveJob{sequence: "1"}
	for want := 1; want <= 2; want++ {
		select {
		case got := <-calls:
			if got != want {
				t.Fatalf("call %d, want %d", got, want)
			}
		case <-time.After(12 * time.Second):
			t.Fatalf("missing authority call %d without a new edit", want)
		}
		expected := partial
		if want == 2 {
			expected = resolved
		}
		deadline := time.Now().Add(time.Second)
		for {
			current.RLock()
			actual := string(current.snapshot.Context)
			current.RUnlock()
			if actual == string(expected) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("metadata not published: %s", actual)
			}
			time.Sleep(time.Millisecond)
		}
	}
}
