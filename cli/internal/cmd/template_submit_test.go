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

	"github.com/cogfoundry-labs/loomloom/cli/internal/platform"
)

func TestTemplatePrecheckFileInsufficientBalanceUsesBoundPlatformMessage(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{Platform: platform.ShengSuanYun}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/officialTemplates/text-v1:precheckWorkbook" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"estimatedTotalCostT":119350,"balanceCheck":{"currency":"CNY","availableBalance":0,"isSufficient":false}}`))
	}))
	defer server.Close()

	workbookPath := filepath.Join(t.TempDir(), "input.xlsx")
	if err := os.WriteFile(workbookPath, []byte("workbook"), 0o644); err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newTemplatePrecheckFileCmd(opts)
	cmd.SetArgs([]string{"text-v1", workbookPath})

	err := cmd.Execute()
	if err == nil || err.Error() != insufficientShengSuanYunBalanceMessage {
		t.Fatalf("error=%v want fixed ShengSuanYun balance message", err)
	}
}

func TestTemplateSubmitFileUsesProvidedClientRequestID(t *testing.T) {
	var submitBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1:validateWorkbook":
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/loom/v1/officialTemplates/text-v1:runWorkbook":
			if err := json.NewDecoder(r.Body).Decode(&submitBody); err != nil {
				t.Fatalf("decode submit body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"runId":"run-1","status":"queued","acceptedAtUnix":1}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	workbookPath := filepath.Join(t.TempDir(), "input.xlsx")
	if err := os.WriteFile(workbookPath, []byte("workbook"), 0o644); err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newTemplateSubmitFileCmd(opts)
	cmd.SetArgs([]string{"text-v1", workbookPath, "--client-request-id", "stable-request-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("template submit-file error = %v", err)
	}
	if submitBody["clientRequestId"] != "stable-request-1" {
		t.Fatalf("clientRequestId=%v want stable-request-1", submitBody["clientRequestId"])
	}
}

func TestTemplateSubmitFilePrintsGeneratedClientRequestID(t *testing.T) {
	var submitBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1:validateWorkbook":
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/loom/v1/officialTemplates/text-v1:runWorkbook":
			if err := json.NewDecoder(r.Body).Decode(&submitBody); err != nil {
				t.Fatalf("decode submit body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"runId":"run-1","status":"queued","acceptedAtUnix":1}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	workbookPath := filepath.Join(t.TempDir(), "input.xlsx")
	if err := os.WriteFile(workbookPath, []byte("workbook"), 0o644); err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newTemplateSubmitFileCmd(opts)
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"text-v1", workbookPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("template submit-file error = %v", err)
	}
	generated, ok := submitBody["clientRequestId"].(string)
	if !ok || !strings.HasPrefix(generated, "loomloom-cli-") {
		t.Fatalf("clientRequestId=%#v want generated loomloom-cli-*", submitBody["clientRequestId"])
	}
	if !strings.Contains(stderr.String(), "clientRequestId: "+generated) {
		t.Fatalf("stderr=%q want generated clientRequestId", stderr.String())
	}
}

func TestTemplateSubmitFilePrintsGeneratedClientRequestIDBeforeRequestFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1:validateWorkbook":
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/loom/v1/officialTemplates/text-v1:runWorkbook":
			http.Error(w, `{"error":"temporary failure"}`, http.StatusServiceUnavailable)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	workbookPath := filepath.Join(t.TempDir(), "input.xlsx")
	if err := os.WriteFile(workbookPath, []byte("workbook"), 0o644); err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newTemplateSubmitFileCmd(opts)
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"text-v1", workbookPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("template submit-file error = nil, want request failure")
	}
	if !strings.Contains(stderr.String(), "clientRequestId: loomloom-cli-") {
		t.Fatalf("stderr=%q want generated clientRequestId before request failure", stderr.String())
	}
}
