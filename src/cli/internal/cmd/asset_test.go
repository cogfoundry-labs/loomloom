package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAssetListUsesProductEndpointWithoutUserID(t *testing.T) {
	var requestedPath string
	var requestedQuery string
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		token:   "secret",
		timeout: time.Second,
	}
	cmd := newAssetListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("asset list command error = %v", err)
	}
	if requestedPath != "/loom/v1/users/me/executables" {
		t.Fatalf("path=%q want Product API assets endpoint", requestedPath)
	}
	if requestedQuery != "" {
		t.Fatalf("query=%q want no identity query", requestedQuery)
	}
	if authorization != "Bearer secret" {
		t.Fatalf("authorization=%q want bearer token", authorization)
	}
	if !strings.Contains(out.String(), `"items": []`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestAssetListSendsPageSize(t *testing.T) {
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newAssetListCmd(opts)
	cmd.SetArgs([]string{"--page-size", "25"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("asset list command error = %v", err)
	}
	if requestedQuery != "pageSize=25" {
		t.Fatalf("query=%q want pageSize=25", requestedQuery)
	}
}
