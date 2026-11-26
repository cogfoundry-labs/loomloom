package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeBaseURLRequiresExplicitServer(t *testing.T) {
	_, err := normalizeBaseURL("")
	if err == nil {
		t.Fatal("expected empty server URL to fail")
	}
}

func TestNormalizeBaseURLAddsScheme(t *testing.T) {
	got, err := normalizeBaseURL("api.cogfoundry.example/loom/v1")
	if err != nil {
		t.Fatalf("normalizeBaseURL failed: %v", err)
	}
	want := "https://api.cogfoundry.example/loom/v1"
	if got != want {
		t.Fatalf("normalizeBaseURL = %q, want %q", got, want)
	}
}

func TestNormalizeBaseURLRejectsRemoteHTTP(t *testing.T) {
	_, err := normalizeBaseURL("http://api.cogfoundry.example/loom/v1")
	if err == nil || !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("error=%v want remote HTTP rejection", err)
	}
}

func TestEndpointUsesConfiguredBaseURL(t *testing.T) {
	c, err := New(Config{BaseURL: "https://api-test.cogfoundry.example/loom/v1"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	got := c.endpoint("/templates")
	want := "https://api-test.cogfoundry.example/loom/v1/templates"
	if got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestEndpointNormalizesMissingLeadingSlash(t *testing.T) {
	c, err := New(Config{BaseURL: "https://api-test.cogfoundry.example/loom/v1"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	got := c.endpoint("marketListings")
	want := "https://api-test.cogfoundry.example/loom/v1/marketListings"
	if got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestEndpointDoesNotRewriteAbsolutePath(t *testing.T) {
	c, err := New(Config{BaseURL: "https://api-test.cogfoundry.example/loom/v1"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	got := c.endpoint("/officialTemplates")
	want := "https://api-test.cogfoundry.example/loom/v1/officialTemplates"
	if got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestVerboseHTTPLogsExcludeTokenQueryAndBody(t *testing.T) {
	const token = "secret-token-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Fatalf("Authorization=%q", got)
		}
		w.Header().Set("X-Request-ID", "request-123")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	var logs bytes.Buffer
	c, err := New(Config{
		BaseURL:   server.URL,
		Token:     token,
		Verbose:   true,
		LogWriter: &logs,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	var response map[string]any
	err = c.PostProductJSONWithQuery(
		context.Background(),
		"/templates/template-1:run",
		url.Values{"sensitive": []string{"query-secret"}},
		map[string]string{"prompt": "body-secret"},
		&response,
	)
	if err != nil {
		t.Fatalf("PostProductJSONWithQuery failed: %v", err)
	}

	got := logs.String()
	for _, want := range []string{
		"[debug] POST /templates/template-1:run",
		"response status=200",
		"request_id=request-123",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("logs=%q want %q", got, want)
		}
	}
	for _, secret := range []string{token, "query-secret", "body-secret", "Authorization"} {
		if strings.Contains(got, secret) {
			t.Fatalf("logs contain sensitive value %q: %s", secret, got)
		}
	}
}

func TestHTTPLogsAreDisabledByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	var logs bytes.Buffer
	c, err := New(Config{BaseURL: server.URL, LogWriter: &logs})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	var response map[string]any
	if err := c.GetJSON(context.Background(), "/health", &response); err != nil {
		t.Fatalf("GetJSON failed: %v", err)
	}
	if logs.Len() != 0 {
		t.Fatalf("logs=%q want no default logs", logs.String())
	}
}

func TestOnSuccessReceivesRequestMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	var got SuccessMeta
	c, err := New(Config{
		BaseURL: server.URL + "/loom/v1",
		Token:   "token-1",
		OnSuccess: func(meta SuccessMeta) {
			got = meta
		},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	var response map[string]any
	if err := c.GetProductJSON(context.Background(), "/users/me/executables", &response); err != nil {
		t.Fatalf("GetProductJSON failed: %v", err)
	}
	if got.Method != http.MethodGet {
		t.Fatalf("method=%q want GET", got.Method)
	}
	if got.Path != "/loom/v1/users/me/executables" {
		t.Fatalf("path=%q want /loom/v1/users/me/executables", got.Path)
	}
	if !got.Authed {
		t.Fatal("authed=false want true")
	}
}

func TestDecodeResponseReturnsRequestError(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
	}
	var out map[string]any
	err := decodeResponse(resp, &out)
	var requestErr RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("error=%T %v want RequestError", err, err)
	}
	if requestErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", requestErr.StatusCode)
	}
}

func TestOnSuccessWaitsForJSONDecode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	called := false
	c, err := New(Config{
		BaseURL: server.URL + "/loom/v1",
		Token:   "token-1",
		OnSuccess: func(meta SuccessMeta) {
			called = true
		},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	var response map[string]any
	if err := c.GetProductJSON(context.Background(), "/users/me/executables", &response); err == nil {
		t.Fatal("GetProductJSON error = nil, want decode error")
	}
	if called {
		t.Fatal("OnSuccess called before successful JSON decode")
	}
}

func TestClientRefusesCrossServerRedirect(t *testing.T) {
	targetCalled := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/loom/v1/users/me/executables", http.StatusFound)
	}))
	defer origin.Close()

	c, err := New(Config{BaseURL: origin.URL + "/loom/v1", Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	err = c.GetProductJSON(context.Background(), "/users/me/executables", &response)
	if err == nil || !strings.Contains(err.Error(), "refusing cross-server redirect") {
		t.Fatalf("error=%v want redirect rejection", err)
	}
	if targetCalled {
		t.Fatal("redirect target was called")
	}
}

func TestGetBinaryReturnsRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	c, err := New(Config{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	_, err = c.GetBinary(context.Background(), "/version")
	var requestErr RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("error=%T %v want RequestError", err, err)
	}
	if requestErr.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d want 403", requestErr.StatusCode)
	}
}

func TestPostStreamFileStreamsRawBytesWithHeaders(t *testing.T) {
	payload := []byte("fake-mp4-binary-payload-0123456789")
	sum := sha256.Sum256(payload)
	wantSHA := hex.EncodeToString(sum[:])

	dir := t.TempDir()
	filePath := filepath.Join(dir, "原始视频 clip 1.mp4")
	if err := os.WriteFile(filePath, payload, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	var gotMethod, gotPath, gotContentType, gotFilename, gotSHA, gotReqID, gotAuth string
	var gotContentLength int64
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotFilename = r.Header.Get("X-Filename")
		gotSHA = r.Header.Get("X-Content-SHA256")
		gotReqID = r.Header.Get("X-Client-Request-Id")
		gotAuth = r.Header.Get("Authorization")
		gotContentLength = r.ContentLength
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"inputAssetId":"asset-123","filename":"原始视频 clip 1.mp4","mimeType":"video/mp4","sizeBytes":34,"uploadedAt":1700000000}`))
	}))
	defer server.Close()

	var successMeta SuccessMeta
	c, err := New(Config{
		BaseURL: server.URL + "/loom/v1",
		Token:   "tok-stream",
		OnSuccess: func(meta SuccessMeta) {
			successMeta = meta
		},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var resp struct {
		InputAssetID string `json:"inputAssetId"`
	}
	err = c.PostStreamFile(context.Background(), "/inputAssets:uploadStream", filePath, StreamUploadOptions{
		ContentType:     "video/mp4",
		ClientRequestID: "req-idem-1",
	}, &resp)
	if err != nil {
		t.Fatalf("PostStreamFile failed: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method=%q want POST", gotMethod)
	}
	if gotPath != "/loom/v1/inputAssets:uploadStream" {
		t.Fatalf("path=%q want /loom/v1/inputAssets:uploadStream", gotPath)
	}
	if gotContentType != "video/mp4" {
		t.Fatalf("Content-Type=%q want video/mp4", gotContentType)
	}
	unescaped, err := url.PathUnescape(gotFilename)
	if err != nil || unescaped != "原始视频 clip 1.mp4" {
		t.Fatalf("X-Filename=%q unescaped=%q err=%v", gotFilename, unescaped, err)
	}
	if gotSHA != wantSHA {
		t.Fatalf("X-Content-SHA256=%q want %q", gotSHA, wantSHA)
	}
	if gotReqID != "req-idem-1" {
		t.Fatalf("X-Client-Request-Id=%q want req-idem-1", gotReqID)
	}
	if gotAuth != "Bearer tok-stream" {
		t.Fatalf("Authorization=%q want Bearer tok-stream", gotAuth)
	}
	if gotContentLength != int64(len(payload)) {
		t.Fatalf("Content-Length=%d want %d", gotContentLength, len(payload))
	}
	if !bytes.Equal(gotBody, payload) {
		t.Fatalf("body=%q want %q", gotBody, payload)
	}
	if resp.InputAssetID != "asset-123" {
		t.Fatalf("InputAssetID=%q want asset-123", resp.InputAssetID)
	}
	if !successMeta.Authed || successMeta.Method != http.MethodPost {
		t.Fatalf("unexpected successMeta=%+v", successMeta)
	}
}

func TestPostStreamFileRejectsEmptyAndDirectory(t *testing.T) {
	c, err := New(Config{BaseURL: "https://api.cogfoundry.example/loom/v1"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	dir := t.TempDir()
	emptyFile := filepath.Join(dir, "empty.mp4")
	if err := os.WriteFile(emptyFile, nil, 0o600); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	if err := c.PostStreamFile(context.Background(), "/inputAssets:uploadStream", emptyFile, StreamUploadOptions{ContentType: "video/mp4"}, nil); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty file err=%v want empty error", err)
	}
	if err := c.PostStreamFile(context.Background(), "/inputAssets:uploadStream", dir, StreamUploadOptions{ContentType: "video/mp4"}, nil); err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("dir err=%v want directory error", err)
	}
}
