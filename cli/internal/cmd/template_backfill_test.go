package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTemplateBackfillResultsCmdDownloadsServerResultWorkbook(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		_, _ = w.Write([]byte("server result workbook"))
	}))
	defer server.Close()

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.xlsx")
	if err := os.WriteFile(inputPath, []byte("old workbook"), 0o644); err != nil {
		t.Fatalf("write input workbook: %v", err)
	}

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newTemplateBackfillResultsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"run_123", inputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("backfill-results command error = %v", err)
	}
	if requestedPath != "/loom/v1/users/me/runs/run_123/resultWorkbook" {
		t.Fatalf("path=%q want result-workbook endpoint", requestedPath)
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read backfilled workbook: %v", err)
	}
	if string(data) != "server result workbook" {
		t.Fatalf("backfilled bytes=%q", string(data))
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output JSON: %v", err)
	}
	if payload["outputFile"] != inputPath {
		t.Fatalf("outputFile=%v want %s", payload["outputFile"], inputPath)
	}
}

func TestResolveBackfillOutputPath_DefaultsToSameWorkbook(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "filled.xlsx")
	got, err := resolveBackfillOutputPath("", inputPath)
	if err != nil {
		t.Fatalf("resolveBackfillOutputPath returned error: %v", err)
	}
	want, err := filepath.Abs(inputPath)
	if err != nil {
		t.Fatalf("filepath.Abs returned error: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected backfill output path: got %q want %q", got, want)
	}
}
