package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOrchestrationInputUploadUsesProductAPI(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "rows.jsonl")
	content := []byte("{\"prompt\":\"hello\"}\n")
	if err := os.WriteFile(inputPath, content, 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	var request uploadOrchestrationInputRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/orchestrationInputs:upload" {
			t.Fatalf("path=%q want orchestration input upload endpoint", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"inputFileId":"ec1015c0-5078-4409-84b5-b46ddc3e9312",
			"filename":"rows.jsonl",
			"rowCount":1,
			"uploadedAt":1782179085
		}`))
	}))
	defer server.Close()

	var logs bytes.Buffer
	opts := &rootOptions{
		server:    server.URL + "/loom/v1",
		timeout:   time.Second,
		verbose:   true,
		logWriter: &logs,
	}
	cmd := newOrchestrationInputUploadCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{inputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("orchestration-input upload error = %v", err)
	}
	if request.Filename != "rows.jsonl" {
		t.Fatalf("filename=%q want rows.jsonl", request.Filename)
	}
	if !bytes.Equal(request.Content, content) {
		t.Fatalf("content=%q want %q", request.Content, content)
	}
	if !strings.Contains(out.String(), "input_file_id\tec1015c0-5078-4409-84b5-b46ddc3e9312") {
		t.Fatalf("unexpected output: %s", out.String())
	}
	for _, want := range []string{
		"uploading filename=rows.jsonl size_bytes=19",
		"completed input_file_id=ec1015c0-5078-4409-84b5-b46ddc3e9312 row_count=1",
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("logs=%q want %q", logs.String(), want)
		}
	}
}

func TestOrchestrationInputUploadRejectsNonJSONL(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "rows.json")
	if err := os.WriteFile(inputPath, []byte(`{"prompt":"hello"}`), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	cmd := newOrchestrationInputUploadCmd(&rootOptions{})
	cmd.SetArgs([]string{inputPath})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "must be a .jsonl file") {
		t.Fatalf("error=%v want .jsonl validation", err)
	}
}

func TestOrchestrationInputUploadRejectsEmptyInputFileID(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "rows.jsonl")
	if err := os.WriteFile(inputPath, []byte("{\"prompt\":\"hello\"}\n"), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"filename":"rows.jsonl","rowCount":1}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newOrchestrationInputUploadCmd(opts)
	cmd.SetArgs([]string{inputPath})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "empty inputFileId") {
		t.Fatalf("error=%v want empty inputFileId rejection", err)
	}
}
