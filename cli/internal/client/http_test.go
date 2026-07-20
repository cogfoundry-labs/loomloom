package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
