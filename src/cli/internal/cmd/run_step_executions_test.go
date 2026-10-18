package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const stepExecutionsFixture = `{
	"items": [{
		"executionId": "code-1", "taskId": "task-1", "sourceRowIndex": 0, "stepId": "stp_code01", "executionIndex": 0,
		"runtimeKind": "code", "status": "failed", "errorCode": "CODE_USER_ERROR", "errorMessage": "ZeroDivisionError: division by zero",
		"input": {"version": 1, "values": [{"portId": "payload", "value": {"score": 91}}]},
		"values": [], "artifacts": [],
		"code": {"attempts": [{"attemptIndex": 1, "status": "failed", "committed": true}], "diagnostic": {"traceback": "user_code line 2"}}
	}, {
		"executionId": "model-1", "taskId": "task-1", "sourceRowIndex": 0, "stepId": "stp_judge01", "loopId": "stp_loop01", "iteration": 2,
		"executionIndex": 0, "runtimeKind": "model", "status": "completed", "inputUnavailable": "not recorded",
		"values": [{"portId": "decision", "value": "accept"}], "artifacts": [],
		"model": {"modelName": "test/model", "promptTokens": 12, "completionTokens": 3, "cost": 34}
	}],
	"loopIterations": [{"taskId": "task-1", "loopId": "stp_loop01", "iteration": 1, "status": "continued",
		"state": {"feedback": "more red"}, "decisionKind": "decision", "decisionStepId": "stp_judge01", "decisionPortId": "decision", "decisionAction": "continue"}],
	"nextPageToken": "2", "totalCount": 3
}`

func TestRunStepExecutionsCmdListsExecutionsAndLoopRounds(t *testing.T) {
	var requestedPath, requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath, requestedQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(stepExecutionsFixture))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "text"}
	cmd := newRunStepExecutionsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"run_123", "--task-id", "task-1", "--page-size", "2", "--page-token", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("step-executions command error = %v", err)
	}
	if requestedPath != "/loom/v1/users/me/runs/run_123/stepExecutions" {
		t.Fatalf("path=%q want stepExecutions endpoint", requestedPath)
	}
	for _, want := range []string{"taskId=task-1", "pageSize=2", "pageToken=1"} {
		if !strings.Contains(requestedQuery, want) {
			t.Fatalf("query %q missing %q", requestedQuery, want)
		}
	}
	for _, want := range []string{"ZeroDivisionError: division by zero", "stp_loop01#2", "test/model tokens=12/3 cost=34",
		"decision=\"accept\"", "(not recorded)", "decision stp_judge01/decision=continue", `{"feedback":"more red"}`, "total_count\t3"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunStepExecutionsJSONKeepsFullDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(stepExecutionsFixture))
	}))
	defer server.Close()

	cmd := newRunStepExecutionsCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"run_123"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("step-executions command error = %v", err)
	}
	var decoded listRunStepExecutionsResponse
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if compactRunJSON(decoded.Items[0].Code.Diagnostic) != `{"traceback":"user_code line 2"}` || decoded.Items[0].Input == nil {
		t.Fatalf("JSON output dropped detail: %s", out.String())
	}
}
