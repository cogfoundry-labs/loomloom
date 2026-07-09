package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArtifactListOnlySendsSupportedQueryParams(t *testing.T) {
	var requestedPath string
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"artifacts":[]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newArtifactListCmd(opts)
	cmd.SetArgs([]string{"run-1", "--page-size", "5"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("artifact list command error = %v", err)
	}
	if requestedPath != "/loom/v1/users/me/runs/run-1/artifacts" {
		t.Fatalf("path=%q want artifacts endpoint", requestedPath)
	}
	if requestedQuery != "pageSize=5" {
		t.Fatalf("query=%q want pageSize=5", requestedQuery)
	}
}

func TestArtifactListRejectsUnsupportedFilterFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for unsupported flags")
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	for _, flag := range []string{"--task-id", "--step-id", "--page-token"} {
		cmd := newArtifactListCmd(opts)
		cmd.SetArgs([]string{"run-1", flag, "value"})

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "unknown flag: "+flag) {
			t.Fatalf("flag %s error=%v want unknown flag", flag, err)
		}
	}
}

func TestArtifactDownloadRejectsUnsupportedFilterFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for unsupported flags")
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	for _, flag := range []string{"--task-id", "--step-id"} {
		cmd := newArtifactDownloadCmd(opts)
		cmd.SetArgs([]string{"run-1", flag, "value"})

		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "unknown flag: "+flag) {
			t.Fatalf("flag %s error=%v want unknown flag", flag, err)
		}
	}
}

func TestArtifactDownloadDoesNotSendUnsupportedPageToken(t *testing.T) {
	var requestedQueries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedQueries = append(requestedQueries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"artifacts": [
				{"artifactId":"artifact-1","filename":"result.txt","mimeType":"text/plain","inlineText":"hello"}
			],
			"nextPageToken": "ignored-by-cli"
		}`))
	}))
	defer server.Close()

	outputDir := t.TempDir()
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newArtifactDownloadCmd(opts)
	cmd.SetArgs([]string{"run-1", "--output-dir", outputDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("artifact download command error = %v", err)
	}
	if len(requestedQueries) != 1 {
		t.Fatalf("requests=%d want 1", len(requestedQueries))
	}
	if requestedQueries[0] != "pageSize=200" {
		t.Fatalf("query=%q want pageSize=200", requestedQueries[0])
	}
	data, err := os.ReadFile(filepath.Join(outputDir, "artifact-1.txt"))
	if err != nil {
		t.Fatalf("read downloaded artifact: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("downloaded data=%q want hello", data)
	}
}
