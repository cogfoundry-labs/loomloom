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

	"github.com/Cogfoundry-ai/loomloom/cli/internal/platform"
)

func TestRunCommandsExposeSplitFlowAndLegacySubmit(t *testing.T) {
	cmd := newRunCmd(&rootOptions{})
	for _, name := range []string{"validate", "precheck", "execute"} {
		found, _, err := cmd.Find([]string{name})
		if err != nil || found.Name() != name {
			t.Fatalf("run %s command missing: found=%v err=%v", name, found, err)
		}
		if found.Hidden {
			t.Fatalf("run %s must be visible", name)
		}
	}
	legacy, _, err := cmd.Find([]string{"submit"})
	if err != nil || legacy.Name() != "submit" {
		t.Fatalf("legacy run submit compatibility command missing: found=%v err=%v", legacy, err)
	}
	if !legacy.Hidden {
		t.Fatal("legacy run submit must be hidden")
	}
}

func TestRunValidateOnlyValidatesRows(t *testing.T) {
	var paths []string
	var validateBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1/schema":
			_, _ = w.Write([]byte(`{
				"templateId":"text-v1",
				"columns":[{"fieldKey":"prompt","headerLabel":"提示词","order":1}]
			}`))
		case "/loom/v1/officialTemplates/text-v1:validateRows":
			if err := json.NewDecoder(r.Body).Decode(&validateBody); err != nil {
				t.Fatalf("decode validate body: %v", err)
			}
			_, _ = w.Write([]byte(`{"valid":true,"rowErrors":[]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := filepath.Join(t.TempDir(), "rows.json")
	if err := os.WriteFile(inputPath, []byte(`[{"prompt":"hello"}]`), 0o644); err != nil {
		t.Fatalf("write rows file: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newRunValidateCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"text-v1", "--file", inputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run validate command error = %v", err)
	}
	wantPaths := []string{
		"/loom/v1/officialTemplates/text-v1/schema",
		"/loom/v1/officialTemplates/text-v1:validateRows",
	}
	if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("paths=%v want %v", paths, wantPaths)
	}
	rows, ok := validateBody["rows"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("rows=%#v want one row", validateBody["rows"])
	}
	if row := rows[0].(map[string]any); row["提示词"] != "hello" {
		t.Fatalf("row=%#v want remapped header label", row)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode validate output: %v\noutput=%s", err, out.String())
	}
	if result["templateId"] != "text-v1" || result["inputPath"] != inputPath || result["rowCount"] != float64(1) {
		t.Fatalf("validate metadata=%#v", result)
	}
	if _, exists := result["valid"]; exists {
		t.Fatalf("valid must remain nested under validation: %#v", result)
	}
	validation, ok := result["validation"].(map[string]any)
	if !ok {
		t.Fatalf("validation=%#v want object", result["validation"])
	}
	if validation["valid"] != true {
		t.Fatalf("validation.valid=%#v want true", validation["valid"])
	}
	rowErrors, ok := validation["rowErrors"].([]any)
	if !ok || len(rowErrors) != 0 {
		t.Fatalf("validation.rowErrors=%#v want empty array", validation["rowErrors"])
	}
}

func TestRunValidateFailureReturnsNestedRowErrors(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1/schema":
			_, _ = w.Write([]byte(`{"templateId":"text-v1"}`))
		case "/loom/v1/officialTemplates/text-v1:validateRows":
			_, _ = w.Write([]byte(`{
				"valid":false,
				"rowErrors":[
					{"rowIndex":0,"fieldKey":"prompt","error":"required"},
					{"rowIndex":1,"fieldKey":"prompt","error":"too long"}
				]
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := filepath.Join(t.TempDir(), "rows.json")
	if err := os.WriteFile(inputPath, []byte(`[{"prompt":""},{"prompt":"long"}]`), 0o644); err != nil {
		t.Fatalf("write rows file: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newRunValidateCmd(opts)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"text-v1", "--file", inputPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("run validate error = nil, want validation failure")
	}
	if !strings.Contains(err.Error(), "row 0 field prompt: required") {
		t.Fatalf("error=%q want first row validation error", err)
	}
	wantPaths := []string{
		"/loom/v1/officialTemplates/text-v1/schema",
		"/loom/v1/officialTemplates/text-v1:validateRows",
	}
	if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("paths=%v want %v", paths, wantPaths)
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode validate output: %v\noutput=%s", err, out.String())
	}
	validation, ok := result["validation"].(map[string]any)
	if !ok {
		t.Fatalf("validation=%#v want object", result["validation"])
	}
	if validation["valid"] != false {
		t.Fatalf("validation.valid=%#v want false", validation["valid"])
	}
	rowErrors, ok := validation["rowErrors"].([]any)
	if !ok || len(rowErrors) != 2 {
		t.Fatalf("validation.rowErrors=%#v want two errors", validation["rowErrors"])
	}
}

func TestRunPrecheckOnlyEstimatesRows(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1/schema":
			_, _ = w.Write([]byte(`{"templateId":"text-v1"}`))
		case "/loom/v1/officialTemplates/text-v1:precheckRows":
			_, _ = w.Write([]byte(`{
				"estimatedTotalCostT":119350,
				"balanceCheck":{
					"currency":"CNY",
					"availableBalance":200000,
					"isSufficient":true
				}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := filepath.Join(t.TempDir(), "rows.jsonl")
	if err := os.WriteFile(inputPath, []byte("{\"prompt\":\"hello\"}\n"), 0o644); err != nil {
		t.Fatalf("write rows file: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newRunPrecheckCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"text-v1", "--file", inputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run precheck command error = %v", err)
	}
	wantPaths := []string{
		"/loom/v1/officialTemplates/text-v1/schema",
		"/loom/v1/officialTemplates/text-v1:precheckRows",
	}
	if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("paths=%v want %v", paths, wantPaths)
	}
	assertContainsAll(t, out.String(), `"estimatedTotalCostT": 119350`, `"currency": "CNY"`)
	assertContainsNone(t, out.String(), `"runId"`)
}

func TestRunPrecheckInsufficientBalanceUsesCurrencyFormat(t *testing.T) {
	isolateCmdConfigHome(t)
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1/schema":
			_, _ = w.Write([]byte(`{"templateId":"text-v1"}`))
		case "/loom/v1/officialTemplates/text-v1:precheckRows":
			_, _ = w.Write([]byte(`{
				"estimatedTotalCostT":119350,
				"balanceCheck":{
					"currency":"CNY",
					"availableBalance":1000,
					"isSufficient":false
				}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := filepath.Join(t.TempDir(), "rows.json")
	if err := os.WriteFile(inputPath, []byte(`[{"prompt":"hello"}]`), 0o644); err != nil {
		t.Fatalf("write rows file: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newRunPrecheckCmd(opts)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"text-v1", "--file", inputPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("run precheck error = nil, want insufficient balance")
	}
	if !strings.Contains(err.Error(), "estimated_cost=CNY 0.0119350") ||
		!strings.Contains(err.Error(), "available=CNY 0.0001000") {
		t.Fatalf("error=%q want currency-aware amounts", err)
	}
	if strings.Contains(err.Error(), "¥") {
		t.Fatalf("error=%q must not use legacy yen-style symbol", err)
	}
	wantPaths := []string{
		"/loom/v1/officialTemplates/text-v1/schema",
		"/loom/v1/officialTemplates/text-v1:precheckRows",
	}
	if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("paths=%v want %v", paths, wantPaths)
	}
}

func TestRunExecuteOnlySubmitsRows(t *testing.T) {
	var paths []string
	var runBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1/schema":
			_, _ = w.Write([]byte(`{"templateId":"text-v1"}`))
		case "/loom/v1/officialTemplates/text-v1:runRows":
			if err := json.NewDecoder(r.Body).Decode(&runBody); err != nil {
				t.Fatalf("decode run body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"runId":"run-1","status":"pending","acceptedAtUnix":1782180000}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := filepath.Join(t.TempDir(), "rows.json")
	if err := os.WriteFile(inputPath, []byte(`[{"prompt":"hello"}]`), 0o644); err != nil {
		t.Fatalf("write rows file: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newRunExecuteCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"text-v1", "--file", inputPath, "--client-request-id", "req-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run execute command error = %v", err)
	}
	wantPaths := []string{
		"/loom/v1/officialTemplates/text-v1/schema",
		"/loom/v1/officialTemplates/text-v1:runRows",
	}
	if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("paths=%v want %v", paths, wantPaths)
	}
	if runBody["clientRequestId"] != "req-1" {
		t.Fatalf("clientRequestId=%#v want req-1", runBody["clientRequestId"])
	}
	assertContainsAll(t, out.String(), `"runId": "run-1"`, `"clientRequestId": "req-1"`)
	assertContainsNone(t, out.String(), `"estimatedTotalCostT"`, `"validation"`)
}

func TestRunExecutePrintsGeneratedClientRequestIDBeforeFailure(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1/schema":
			_, _ = w.Write([]byte(`{"templateId":"text-v1"}`))
		case "/loom/v1/officialTemplates/text-v1:runRows":
			http.Error(w, `{"error":"temporary failure"}`, http.StatusServiceUnavailable)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := filepath.Join(t.TempDir(), "rows.json")
	if err := os.WriteFile(inputPath, []byte(`[{"prompt":"hello"}]`), 0o644); err != nil {
		t.Fatalf("write rows file: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newRunExecuteCmd(opts)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"text-v1", "--file", inputPath})

	if err := cmd.Execute(); err == nil {
		t.Fatal("run execute error = nil, want request failure")
	}
	if !strings.Contains(stderr.String(), "clientRequestId: loomloom-cli-") {
		t.Fatalf("stderr=%q want generated clientRequestId before request failure", stderr.String())
	}
	wantPaths := []string{
		"/loom/v1/officialTemplates/text-v1/schema",
		"/loom/v1/officialTemplates/text-v1:runRows",
	}
	if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("paths=%v want %v", paths, wantPaths)
	}
}

func TestRunSubmitSendsFlatRowsAndClientRequestID(t *testing.T) {
	var validateBody map[string]any
	var precheckBody map[string]any
	var runBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1/schema":
			_, _ = w.Write([]byte(`{
				"templateId":"text-v1",
				"columns":[
					{"fieldKey":"text_prompts","headerLabel":"文本提示词","order":1},
					{"fieldKey":"writing_requirements","headerLabel":"写作要求","order":2}
				]
			}`))
		case "/loom/v1/officialTemplates/text-v1:validateRows":
			if err := json.NewDecoder(r.Body).Decode(&validateBody); err != nil {
				t.Fatalf("decode validate body: %v", err)
			}
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/loom/v1/officialTemplates/text-v1:precheckRows":
			if err := json.NewDecoder(r.Body).Decode(&precheckBody); err != nil {
				t.Fatalf("decode precheck body: %v", err)
			}
			_, _ = w.Write([]byte(`{"estimatedTotalCostT":0}`))
		case "/loom/v1/officialTemplates/text-v1:runRows":
			if err := json.NewDecoder(r.Body).Decode(&runBody); err != nil {
				t.Fatalf("decode run body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"runId":"run-1","status":"pending","acceptedAtUnix":1782180000}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := filepath.Join(t.TempDir(), "rows.json")
	if err := os.WriteFile(inputPath, []byte(`[{"text_prompts":"hello","writing_requirements":"short"}]`), 0o644); err != nil {
		t.Fatalf("write rows file: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newRunSubmitCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"text-v1", "--file", inputPath, "--client-request-id", "req-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run submit command error = %v", err)
	}
	if !strings.Contains(out.String(), `"estimatedTotalCostT": 0`) {
		t.Fatalf("output=%s want estimatedTotalCostT", out.String())
	}
	if strings.Contains(out.String(), `"estimatedTotalCost":`) {
		t.Fatalf("output=%s must not emit estimatedTotalCost", out.String())
	}
	for name, body := range map[string]map[string]any{
		"validate": validateBody,
		"precheck": precheckBody,
		"run":      runBody,
	} {
		rows, ok := body["rows"].([]any)
		if !ok || len(rows) != 1 {
			t.Fatalf("%s rows=%#v want one row", name, body["rows"])
		}
		row, ok := rows[0].(map[string]any)
		if !ok {
			t.Fatalf("%s row=%#v want object", name, rows[0])
		}
		if _, ok := row["values"]; ok {
			t.Fatalf("%s row should not contain values wrapper: %#v", name, row)
		}
		if row["文本提示词"] != "hello" || row["写作要求"] != "short" {
			t.Fatalf("%s row=%#v want remapped header labels", name, row)
		}
		if body["clientRequestId"] != "req-1" {
			t.Fatalf("%s clientRequestId=%v want req-1", name, body["clientRequestId"])
		}
	}
}

func TestRunSubmitPrintsGeneratedClientRequestIDBeforeRunFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1/schema":
			_, _ = w.Write([]byte(`{"templateId":"text-v1"}`))
		case "/loom/v1/officialTemplates/text-v1:validateRows":
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/loom/v1/officialTemplates/text-v1:precheckRows":
			_, _ = w.Write([]byte(`{"estimatedTotalCostT":0}`))
		case "/loom/v1/officialTemplates/text-v1:runRows":
			http.Error(w, `{"error":"temporary failure"}`, http.StatusServiceUnavailable)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := filepath.Join(t.TempDir(), "rows.json")
	if err := os.WriteFile(inputPath, []byte(`[{"prompt":"hello"}]`), 0o644); err != nil {
		t.Fatalf("write rows file: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newRunSubmitCmd(opts)
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"text-v1", "--file", inputPath})

	if err := cmd.Execute(); err == nil {
		t.Fatal("run submit error = nil, want request failure")
	}
	if !strings.Contains(stderr.String(), "clientRequestId: loomloom-cli-") {
		t.Fatalf("stderr=%q want generated clientRequestId before request failure", stderr.String())
	}
}

func TestRunSubmitInsufficientBalanceUsesCurrencyFormat(t *testing.T) {
	isolateCmdConfigHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1/schema":
			_, _ = w.Write([]byte(`{"templateId":"text-v1"}`))
		case "/loom/v1/officialTemplates/text-v1:validateRows":
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/loom/v1/officialTemplates/text-v1:precheckRows":
			_, _ = w.Write([]byte(`{
				"estimatedTotalCostT":119350,
				"balanceCheck":{
					"currency":"CNY",
					"availableBalance":1000,
					"isSufficient":false
				}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := filepath.Join(t.TempDir(), "rows.json")
	if err := os.WriteFile(inputPath, []byte(`[{"prompt":"hello"}]`), 0o644); err != nil {
		t.Fatalf("write rows file: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newRunSubmitCmd(opts)
	cmd.SetArgs([]string{"text-v1", "--file", inputPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("run submit error = nil, want insufficient balance")
	}
	if !strings.Contains(err.Error(), "estimated_cost=CNY 0.0119350") ||
		!strings.Contains(err.Error(), "available=CNY 0.0001000") {
		t.Fatalf("error=%q want currency-aware amounts", err)
	}
	if strings.Contains(err.Error(), "¥") {
		t.Fatalf("error=%q must not use legacy yen-style symbol", err)
	}
}

func TestRunSubmitInsufficientBalanceUsesBoundPlatformMessage(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{Platform: platform.ShengSuanYun}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/officialTemplates/text-v1/schema":
			_, _ = w.Write([]byte(`{"templateId":"text-v1"}`))
		case "/loom/v1/officialTemplates/text-v1:validateRows":
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/loom/v1/officialTemplates/text-v1:precheckRows":
			_, _ = w.Write([]byte(`{
				"estimatedTotalCostT":119350,
				"balanceCheck":{
					"currency":"CNY",
					"availableBalance":1000,
					"isSufficient":false
				}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := filepath.Join(t.TempDir(), "rows.json")
	if err := os.WriteFile(inputPath, []byte(`[{"prompt":"hello"}]`), 0o644); err != nil {
		t.Fatalf("write rows file: %v", err)
	}

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newRunSubmitCmd(opts)
	cmd.SetArgs([]string{"text-v1", "--file", inputPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("run submit error = nil, want insufficient balance")
	}
	if err.Error() != insufficientShengSuanYunBalanceMessage {
		t.Fatalf("error=%q want fixed ShengSuanYun balance message", err)
	}
}

func TestRunResultRowsCmdUsesSnapshotResultEndpoint(t *testing.T) {
	var requestedPath string
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"rows": [
				{
					"rowIndex": 0,
					"status": "completed",
					"inputJson": "{\"prompt\":\"hello\"}",
					"artifacts": [{"artifactId":"art_1","mimeType":"text/plain"}]
				}
			],
			"nextPageToken": "2",
			"totalCount": 3
		}`))
	}))
	defer server.Close()

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		timeout: time.Second,
		output:  "text",
	}
	cmd := newRunResultRowsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"run_123", "--page-size", "2", "--page-token", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("result-rows command error = %v", err)
	}
	if requestedPath != "/loom/v1/users/me/runs/run_123/resultRows" {
		t.Fatalf("path=%q want result-rows endpoint", requestedPath)
	}
	for _, want := range []string{"pageSize=2", "pageToken=1"} {
		if !strings.Contains(requestedQuery, want) {
			t.Fatalf("query %q missing %q", requestedQuery, want)
		}
	}
	if !strings.Contains(out.String(), "completed") || !strings.Contains(out.String(), "total_count\t3") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestRunResultRowsTextRedactsSignedAccessURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{
					"rowIndex": 0,
					"status": "completed",
					"artifacts": [{
						"artifactId":"art_1",
						"mimeType":"application/pdf",
						"accessUrl":"https://signed.example/result.pdf?token=secret"
					}]
				}
			]
		}`))
	}))
	defer server.Close()

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		timeout: time.Second,
		output:  "text",
	}
	cmd := newRunResultRowsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"run_123"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("result-rows command error = %v", err)
	}
	if !strings.Contains(out.String(), "download_url_available") {
		t.Fatalf("output=%s want download_url_available marker", out.String())
	}
	for _, forbidden := range []string{"signed.example", "token=secret"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("output=%s must not contain signed access URL fragment %q", out.String(), forbidden)
		}
	}
}

func TestRunResultWorkbookCmdDownloadsServerWorkbook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/users/me/runs/run_123/resultWorkbook" {
			t.Fatalf("path=%q want result-workbook endpoint", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="result-run_123.xlsx"`)
		_, _ = w.Write([]byte("xlsx bytes"))
	}))
	defer server.Close()

	outDir := t.TempDir()
	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newRunResultWorkbookCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"run_123", "--output-file", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("result-workbook command error = %v", err)
	}
	target := filepath.Join(outDir, "result-run_123.xlsx")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read result workbook: %v", err)
	}
	if string(data) != "xlsx bytes" {
		t.Fatalf("downloaded bytes=%q", string(data))
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output JSON: %v", err)
	}
	if payload["path"] != target {
		t.Fatalf("output path=%v want %s", payload["path"], target)
	}
}

func TestRunListCmdUsesProductRunsEndpoint(t *testing.T) {
	var requestedPath string
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		timeout: time.Second,
	}
	cmd := newRunListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--status", "running", "--page-size", "10", "--page-token", "next", "--order-by", "createdAt desc"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run list command error = %v", err)
	}
	if requestedPath != "/loom/v1/users/me/runs" {
		t.Fatalf("path=%q want Product API runs endpoint", requestedPath)
	}
	if strings.Contains(requestedQuery, "buyer_user_id=") {
		t.Fatalf("query %q should not include buyer_user_id", requestedQuery)
	}
	for _, want := range []string{"status=running", "pageSize=10", "pageToken=next", "orderBy=created_at_desc"} {
		if !strings.Contains(requestedQuery, want) {
			t.Fatalf("query %q missing %q", requestedQuery, want)
		}
	}
	if !strings.Contains(out.String(), `"items": []`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestNormalizeRunOrderBy(t *testing.T) {
	tests := map[string]string{
		"createdAt desc":    "created_at_desc",
		"created_at asc":    "created_at_asc",
		"updatedAt desc":    "updated_at_desc",
		"updated_at_asc":    "updated_at_asc",
		" UPDATEDAT   ASC ": "updated_at_asc",
	}
	for input, want := range tests {
		got, err := normalizeRunOrderBy(input)
		if err != nil {
			t.Fatalf("normalizeRunOrderBy(%q) error = %v", input, err)
		}
		if got != want {
			t.Fatalf("normalizeRunOrderBy(%q) = %q, want %q", input, got, want)
		}
	}

	if _, err := normalizeRunOrderBy("price desc"); err == nil {
		t.Fatal("normalizeRunOrderBy(price desc) expected an error")
	}
}

func TestRunGetCmdUsesProductRunDetailEndpoint(t *testing.T) {
	var requestedPath string
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"run":{"run_id":"run-1"}}`))
	}))
	defer server.Close()

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		timeout: time.Second,
	}
	cmd := newRunGetCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"run-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run get command error = %v", err)
	}
	if requestedPath != "/loom/v1/users/me/runs/run-1" {
		t.Fatalf("path=%q want Product API run detail endpoint", requestedPath)
	}
	if requestedQuery != "" {
		t.Fatalf("query=%q want no identity query", requestedQuery)
	}
	if !strings.Contains(out.String(), `"run_id": "run-1"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}
