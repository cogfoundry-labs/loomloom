package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/platform"
	"github.com/cogfoundry-labs/loomloom/src/cli/internal/publicinput"
)

func TestMarketPublishPreservesSkillPackageWhenTemplateVersionIsUnchanged(t *testing.T) {
	var requestedPath string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/creators/me/marketListings/listing-1":
			_, _ = w.Write([]byte(`{"publishedVersionId":"listing-version-1"}`))
		case "/loom/v1/creators/me/marketListings/listing-1/versions":
			if r.URL.Query().Get("pageSize") != "500" {
				t.Fatalf("pageSize=%q want 500", r.URL.Query().Get("pageSize"))
			}
			_, _ = w.Write([]byte(`{"items":[{"id":"listing-version-1","templateVersionId":"version-1"}]}`))
		case "/loom/v1/marketListings":
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"listing-1"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingPublishCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"template-1",
		"--listing-id", "listing-1",
		"--template-version-id", "version-1",
		"--display-name", "PRD Review Bot",
		"--description", "Review PRD docs",
		"--task-fixed-fee", "0.5",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market publish command error = %v", err)
	}
	if requestedPath != "/loom/v1/marketListings" {
		t.Fatalf("path=%q want publish endpoint", requestedPath)
	}
	if _, ok := body["creator_user_id"]; ok {
		t.Fatalf("publish body should not include creator_user_id: %#v", body)
	}
	if body["taskFixedFeeT"] != float64(5_000_000) {
		t.Fatalf("taskFixedFeeT=%v want 5000000", body["taskFixedFeeT"])
	}
	if body["displayName"] != "PRD Review Bot" {
		t.Fatalf("displayName=%v want PRD Review Bot", body["displayName"])
	}
	if body["templateId"] != "template-1" {
		t.Fatalf("templateId=%v want template-1", body["templateId"])
	}
	if body["listingId"] != "listing-1" {
		t.Fatalf("listingId=%v want listing-1", body["listingId"])
	}
	if body["templateVersionId"] != "version-1" {
		t.Fatalf("templateVersionId=%v want version-1", body["templateVersionId"])
	}
	if _, ok := body["workflow_definition"]; ok {
		t.Fatalf("publish body should not include workflow_definition: %#v", body)
	}
	if _, ok := body["definition_hash"]; ok {
		t.Fatalf("publish body should not include definition_hash: %#v", body)
	}
	if _, ok := body["skillPackage"]; ok {
		t.Fatalf("publish body should omit skillPackage to preserve the current package: %#v", body)
	}
	if !strings.Contains(out.String(), `"id": "listing-1"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestMarketPublishUsesAutoWhenTemplateVersionChanges(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/creators/me/marketListings/listing-1":
			_, _ = w.Write([]byte(`{"publishedVersionId":"listing-version-1"}`))
		case "/loom/v1/creators/me/marketListings/listing-1/versions":
			_, _ = w.Write([]byte(`{"items":[{"id":"listing-version-1","templateVersionId":"version-old"}]}`))
		case "/loom/v1/marketListings":
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"listing-1"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingPublishCmd(opts)
	cmd.SetArgs([]string{
		"template-1",
		"--listing-id", "listing-1",
		"--template-version-id", "version-new",
		"--display-name", "PRD Review Bot",
		"--task-fixed-fee", "0.5",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market publish command error = %v", err)
	}
	selection, ok := body["skillPackage"].(map[string]any)
	if !ok || selection["mode"] != "auto" {
		t.Fatalf("skillPackage=%#v want auto", body["skillPackage"])
	}
}

func TestMarketPublishSendsArchiveSkillPackageSelection(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"listing-1"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingPublishCmd(opts)
	cmd.SetArgs([]string{
		"template-1", "--template-version-id", "version-1", "--display-name", "PRD Review Bot", "--task-fixed-fee", "0.5",
		"--skill-package-archive-hash", "sha256:archive", "--skill-package-validation-id", "validation-1",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("market publish command error = %v", err)
	}
	selection, ok := body["skillPackage"].(map[string]any)
	if !ok {
		t.Fatalf("skillPackage=%#v want object", body["skillPackage"])
	}
	if selection["mode"] != "archive" || selection["expectedArchiveHash"] != "sha256:archive" || selection["expectedValidationId"] != "validation-1" {
		t.Fatalf("unexpected skill package selection: %#v", selection)
	}
}

func TestMarketPublishRejectsIncompleteSkillPackageSelection(t *testing.T) {
	for _, args := range [][]string{
		{"--skill-package-archive-hash", "sha256:archive"},
		{"--skill-package-validation-id", "validation-1"},
	} {
		cmd := newListingPublishCmd(&rootOptions{server: "https://example.test", timeout: time.Second})
		cmd.SetArgs(append([]string{"template-1", "--template-version-id", "version-1", "--display-name", "PRD Review Bot", "--task-fixed-fee", "0.5"}, args...))
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "must be provided together") {
			t.Fatalf("args=%v error=%v want tuple validation", args, err)
		}
	}
}

func TestMarketPublishTaskFixedFeeConvertsCurrencyUnits(t *testing.T) {
	tests := []struct {
		name string
		fee  string
		want float64
	}{
		{name: "one currency unit", fee: "1", want: 10_000_000},
		{name: "micro amount", fee: "0.000001", want: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":"listing-1"}`))
			}))
			defer server.Close()

			opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
			cmd := newListingPublishCmd(opts)
			cmd.SetArgs([]string{
				"template-1",
				"--template-version-id", "version-1",
				"--display-name", "PRD Review Bot",
				"--task-fixed-fee", tt.fee,
			})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("market publish command error = %v", err)
			}
			if body["taskFixedFeeT"] != tt.want {
				t.Fatalf("taskFixedFeeT=%v want %.0f", body["taskFixedFeeT"], tt.want)
			}
		})
	}
}

func TestMarketPublishDeprecatedTaskFixedFeeTStillWorks(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"listing-1"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingPublishCmd(opts)
	cmd.SetArgs([]string{
		"template-1",
		"--template-version-id", "version-1",
		"--display-name", "PRD Review Bot",
		"--task-fixed-fee-t", "5000000",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market publish command error = %v", err)
	}
	if body["taskFixedFeeT"] != float64(5_000_000) {
		t.Fatalf("taskFixedFeeT=%v want 5000000", body["taskFixedFeeT"])
	}
}

func TestDeprecatedMarketPublishTaskFixedFeeConvertsCurrencyUnits(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"listing-1"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newDeprecatedMarketPublishCmd(opts)
	cmd.SetArgs([]string{
		"--template-id", "template-1",
		"--template-version-id", "version-1",
		"--display-name", "PRD Review Bot",
		"--task-fixed-fee", "0.5",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("deprecated market publish command error = %v", err)
	}
	if body["taskFixedFeeT"] != float64(5_000_000) {
		t.Fatalf("taskFixedFeeT=%v want 5000000", body["taskFixedFeeT"])
	}
}

func TestMarketListSendsProductAPIQueryParams(t *testing.T) {
	var requestedPath string
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketListCmd(opts)
	cmd.SetArgs([]string{
		"--keyword", "prd",
		"--page-size", "5",
		"--page-token", "next",
		"--order-by", "createdAt desc",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market list command error = %v", err)
	}
	if requestedPath != "/loom/v1/marketListings" {
		t.Fatalf("path=%q want market listings endpoint", requestedPath)
	}
	for _, want := range []string{"keyword=prd", "pageSize=5", "pageToken=next", "orderBy=createdAt+desc"} {
		if !strings.Contains(requestedQuery, want) {
			t.Fatalf("query=%q missing %s", requestedQuery, want)
		}
	}
}

func TestMarketListRejectsLimitAndPageSizeTogether(t *testing.T) {
	opts := &rootOptions{server: "https://example.test/loom/v1", timeout: time.Second}
	cmd := newMarketListCmd(opts)
	cmd.SetArgs([]string{"--limit", "5", "--page-size", "10"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--limit and --page-size cannot be used together") {
		t.Fatalf("error=%v want limit/page-size conflict", err)
	}
}

func TestMarketListJSONPreservesUnknownFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items":[{"id":"listing-1","displayName":"Writer","newBackendField":"kept"}],
			"nextPageToken":"next",
			"serverTraceId":"trace-1"
		}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newMarketListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market list command error = %v", err)
	}
	for _, want := range []string{`"serverTraceId": "trace-1"`, `"newBackendField": "kept"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
}

func TestMarketShowUsesDetailEndpoint(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(marketListingDetailBody(t)))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketShowCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market show command error = %v", err)
	}
	if requestedPath != "/loom/v1/marketListings/listing-1" {
		t.Fatalf("path=%q want detail endpoint", requestedPath)
	}
	assertContainsAll(t, out.String(), "listing-1", "CNY 0.000001", "fields:", "Prompt", "prompt", "sample_rows:")
	assertContainsNone(t, out.String(), "task_fixed_fee_t")
}

func TestMarketShowDisplaysEnumChoices(t *testing.T) {
	schema := `{"fields":[{"key":"stage","label":"Funding stage","required":true,"value_type":"enum","enum_values":["Seed","Series A"]}]}`
	body, err := json.Marshal(map[string]any{
		"id":                  "listing-1",
		"displayName":         "Fundraising helper",
		"inputSchemaSnapshot": json.RawMessage(schema),
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketShowCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market show command error = %v", err)
	}
	assertContainsAll(t, out.String(), "choices", `["Seed","Series A"]`)
}

func TestMarketShowSanitizesRemoteSchemaText(t *testing.T) {
	schema, err := json.Marshal(map[string]any{
		"schema_version": "v1\nINJECTED_VERSION",
		"instructions":   []string{"guide\nINJECTED_INSTRUCTION\tcell\x1b[31m"},
		"fields": []map[string]any{{
			"key":         "stage\nINJECTED_KEY",
			"label":       "Funding\tstage\nINJECTED_LABEL",
			"value_type":  "enum\nINJECTED_TYPE",
			"source_kind": "user_input\tINJECTED_SOURCE",
			"enum_values": []string{"Seed\u009b[31m", "Series\u202eA"},
		}},
		"sample_rows": []map[string]string{{"stage": "Seed\u009b[31m\u202e"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"id":                  "listing-1",
		"displayName":         "Fundraising helper",
		"inputSchemaSnapshot": json.RawMessage(schema),
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketShowCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market show command error = %v", err)
	}
	assertContainsAll(t, out.String(),
		"v1 INJECTED_VERSION",
		"guide INJECTED_INSTRUCTION cell [31m",
		"Funding stage INJECTED_LABEL",
		"stage INJECTED_KEY",
		"enum INJECTED_TYPE",
		"user_input INJECTED_SOURCE",
		"Seed [31m",
		"Series A",
	)
	assertContainsNone(t, out.String(), "\nINJECTED_", "\tINJECTED_", "\x1b", "\u009b", "\u202e")
}

func TestPromptMarketInputRowsSanitizesDisplayAndPreservesOriginalKey(t *testing.T) {
	const key = "stage\nINJECTED_KEY"
	var prompt bytes.Buffer
	rows, err := promptMarketInputRows(strings.NewReader("Seed\n"), &prompt, publicinput.Schema{
		Fields: []publicinput.Field{{
			Key:        key,
			Label:      "Funding\tstage\nINJECTED_LABEL\x1b[31m",
			Required:   true,
			ValueType:  "enum",
			EnumValues: []string{"Seed\u009b[31m\u202e", "Series A"},
		}},
	})
	if err != nil {
		t.Fatalf("promptMarketInputRows() error = %v", err)
	}
	assertContainsAll(t, prompt.String(), "Funding stage INJECTED_LABEL [31m", "stage INJECTED_KEY", "Seed [31m", "Series A")
	assertContainsNone(t, prompt.String(), "\nINJECTED_", "\tINJECTED_", "\x1b", "\u009b", "\u202e")
	if len(rows) != 1 || rows[0][key] != "Seed" {
		t.Fatalf("rows = %#v, want value stored under original key %q", rows, key)
	}
}

func TestMarketShowJSONPreservesUnknownFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"listing-1",
			"displayName":"Writer",
			"inputSchemaSnapshot":"{}",
			"newBackendField":{"nested":true}
		}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newMarketShowCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market show command error = %v", err)
	}
	for _, want := range []string{`"newBackendField": {`, `"nested": true`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
}

func TestMarketShowKeepsBasicOutputWhenSchemaInvalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"listing-1","displayName":"Broken","inputSchemaSnapshot":"{"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketShowCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market show command error = %v", err)
	}
	if !strings.Contains(out.String(), "listing-1") || !strings.Contains(out.String(), "input_schema_error") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestMarketPublishRequiresExplicitTaskFixedFee(t *testing.T) {
	opts := &rootOptions{server: "https://example.test", timeout: time.Second}
	cmd := newListingPublishCmd(opts)
	cmd.SetArgs([]string{
		"template-1",
		"--template-version-id", "version-1",
		"--display-name", "PRD Review Bot",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--task-fixed-fee") {
		t.Fatalf("error=%v want task fee required", err)
	}
}

func TestMarketPublishRejectsTaskFixedFeeConflict(t *testing.T) {
	opts := &rootOptions{server: "https://example.test", timeout: time.Second}
	cmd := newListingPublishCmd(opts)
	cmd.SetArgs([]string{
		"template-1",
		"--template-version-id", "version-1",
		"--display-name", "PRD Review Bot",
		"--task-fixed-fee", "0.5",
		"--task-fixed-fee-t", "5000000",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("error=%v want task fee conflict", err)
	}
}

func TestDeprecatedMarketCommandsRemainAvailable(t *testing.T) {
	cmd := newMarketCmd(&rootOptions{})
	if found, _, err := cmd.Find([]string{"publish"}); err != nil || found.Name() != "publish" {
		t.Fatalf("market publish compatibility command missing: found=%v err=%v", found, err)
	}
	if found, _, err := cmd.Find([]string{"relist"}); err != nil || found.Name() != "relist" {
		t.Fatalf("market relist compatibility command missing: found=%v err=%v", found, err)
	}
}

func TestMarketQuoteSendsOnlyPublicInputRows(t *testing.T) {
	var body map[string]any
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			_, _ = w.Write([]byte(`{"quoteId":"quote-1","currency":"CNY","estimatedBuyerPayableT":10}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{
		"listingVersionId": "lv-1",
		"user_id": 1,
		"buyer_user_id": 13005,
		"creator_user_id": 13004,
		"inputRows": [{"prompt": "review this"}]
	}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketQuoteCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market quote command error = %v", err)
	}
	if len(paths) != 2 || paths[0] != "/loom/v1/marketListings/listing-1" || paths[1] != "/loom/v1/marketListings/listing-1:quote" {
		t.Fatalf("paths=%v want detail then quote", paths)
	}
	for _, field := range []string{"user_id", "buyer_user_id", "creator_user_id", "listingVersionId", "taskInputs", "workflowDefinition", "templateSpec"} {
		if _, ok := body[field]; ok {
			t.Fatalf("quote body should not include %s: %#v", field, body)
		}
	}
	rows, ok := body["inputRows"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("inputRows=%#v want one row", body["inputRows"])
	}
	row := rows[0].(map[string]any)
	if row["prompt"] != "review this" {
		t.Fatalf("row=%#v want prompt", row)
	}
}

func TestMarketQuoteInteractiveShowsEnumChoicesWithoutLocalMembershipValidation(t *testing.T) {
	var body map[string]any
	schema := `{"fields":[{"key":"stage","label":"Funding stage","required":true,"value_type":"enum","enum_values":["Seed","Series A"]}]}`
	listingBody, err := json.Marshal(map[string]any{
		"id":                          "listing-1",
		"displayName":                 "Fundraising helper",
		"listingVersionId":            "lv-1",
		"executionAvailabilityStatus": "available",
		"inputSchemaSnapshot":         json.RawMessage(schema),
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write(listingBody)
		case "/loom/v1/marketListings/listing-1:quote":
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode quote body: %v", err)
			}
			_, _ = w.Write([]byte(`{"quoteId":"quote-1"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketQuoteCmd(opts)
	var prompt bytes.Buffer
	cmd.SetIn(strings.NewReader("Unknown stage\n"))
	cmd.SetErr(&prompt)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market quote command error = %v", err)
	}
	assertContainsAll(t, prompt.String(), "Funding stage", "Allowed values", `["Seed","Series A"]`)
	rows, ok := body["inputRows"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("inputRows=%#v want one row", body["inputRows"])
	}
	row, ok := rows[0].(map[string]any)
	if !ok || row["stage"] != "Unknown stage" {
		t.Fatalf("row=%#v want unvalidated enum value forwarded", rows[0])
	}
}

func TestMarketQuoteInsufficientBalanceUsesBoundPlatformMessage(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{Platform: platform.ShengSuanYun}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			_, _ = w.Write([]byte(`{"quoteId":"quote-1","balanceCheck":{"currency":"CNY","availableBalance":0,"isSufficient":false}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"inputRows":[{"prompt":"review this"}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketQuoteCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath})

	err := cmd.Execute()
	if err == nil || err.Error() != insufficientShengSuanYunBalanceMessage {
		t.Fatalf("error=%v want fixed ShengSuanYun balance message", err)
	}
}

func TestMarketRunInsufficientBalanceDoesNotExecute(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{Platform: platform.ShengSuanYun}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	executeCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			_, _ = w.Write([]byte(`{"quoteId":"quote-1","balanceCheck":{"currency":"CNY","availableBalance":0,"isSufficient":false}}`))
		case "/loom/v1/marketListings/listing-1:execute":
			executeCalled = true
			_, _ = w.Write([]byte(`{"runId":"run-1"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"inputRows":[{"prompt":"review this"}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketRunCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath, "--confirm"})

	err := cmd.Execute()
	if err == nil || err.Error() != insufficientShengSuanYunBalanceMessage {
		t.Fatalf("error=%v want fixed ShengSuanYun balance message", err)
	}
	if executeCalled {
		t.Fatal("execute endpoint was called despite insufficient balance")
	}
}

func TestMarketRunUnknownPlatformInsufficientBalanceDoesNotExecute(t *testing.T) {
	isolateCmdConfigHome(t)
	executeCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			_, _ = w.Write([]byte(`{"quoteId":"quote-1","balanceCheck":{"currency":"CNY","availableBalance":0,"isSufficient":false}}`))
		case "/loom/v1/marketListings/listing-1:execute":
			executeCalled = true
			_, _ = w.Write([]byte(`{"runId":"run-1"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"inputRows":[{"prompt":"review this"}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketRunCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath, "--confirm"})

	err := cmd.Execute()
	if err == nil || err.Error() != "insufficient balance" {
		t.Fatalf("error=%v want generic insufficient balance", err)
	}
	if executeCalled {
		t.Fatal("execute endpoint was called despite insufficient balance")
	}
}

func TestMarketQuoteRejectsListingVersionMismatch(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"listingVersionId":"lv-old","inputRows":[{"prompt":"review this"}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketQuoteCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `listingVersionId "lv-old" does not match current listing version "lv-1"`) {
		t.Fatalf("error=%v want listingVersionId mismatch", err)
	}
	if len(paths) != 1 || paths[0] != "/loom/v1/marketListings/listing-1" {
		t.Fatalf("paths=%v want detail only", paths)
	}
}

func TestMarketQuoteRejectsInternalPayloadFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(marketListingDetailBody(t)))
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"taskInputs":[]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketQuoteCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "taskInputs is an internal Market execution field") {
		t.Fatalf("error=%v want internal field rejection", err)
	}
}

func TestMarketQuoteRejectsNullInputRowValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(marketListingDetailBody(t)))
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"inputRows":[{"prompt":null}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketQuoteCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `inputRows[0] field "prompt": null is not supported`) {
		t.Fatalf("error=%v want null value rejection", err)
	}
}

func TestMarketQuoteTextShowsFormattedAmounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			_, _ = w.Write([]byte(`{
				"quoteId":"quote-1",
				"listingVersionId":"lv-1",
				"currency":"CNY",
				"estimatedExecutionCostT":69300,
				"estimatedExecutionCost":{"amount":"0.0069300","currency":"CNY"},
				"taskFixedFeeT":5000000,
				"taskFixedFee":{"amount":"0.5000000","currency":"CNY"},
				"taskCount":2,
				"estimatedBuyerPayableT":10069300,
				"estimatedBuyerPayable":{"amount":"1.0069300","currency":"CNY"}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"inputRows":[{"prompt":"review this"}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketQuoteCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market quote command error = %v", err)
	}
	assertContainsAll(t, out.String(),
		"CNY 0.5",
		"CNY 1.00693",
		"CNY 0.00693",
	)
	assertContainsNone(t, out.String(),
		"task_fixed_fee_t",
		"estimated_payable_t",
		"estimated_execution_cost_t",
		"10069300",
	)
}

func TestMarketQuoteTextUnknownCurrencyFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			_, _ = w.Write([]byte(`{"quoteId":"quote-1","taskFixedFeeT":5000000,"estimatedBuyerPayableT":5000000}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"inputRows":[{"prompt":"review this"}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketQuoteCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market quote command error = %v", err)
	}
	if !strings.Contains(out.String(), "(currency unknown) 5000000") {
		t.Fatalf("output=%s want currency-unknown fallback", out.String())
	}
}

func TestMarketQuoteJSONPreservesRawFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			_, _ = w.Write([]byte(`{
				"quoteId":"quote-1",
				"currency":"CNY",
				"taskFixedFeeT":5000000,
				"taskFixedFee":{"amount":"0.5000000","currency":"CNY"},
				"estimatedBuyerPayableT":5069300,
				"estimatedBuyerPayable":{"amount":"0.5069300","currency":"CNY"}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"inputRows":[{"prompt":"review this"}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newMarketQuoteCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market quote command error = %v", err)
	}
	for _, want := range []string{
		`"currency": "CNY"`,
		`"taskFixedFee": {`,
		`"estimatedBuyerPayable": {`,
		`"amount": "0.5"`,
		`"amount": "0.50693"`,
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
	assertContainsNone(t, out.String(), "taskFixedFeeT", "estimatedBuyerPayableT")
	if strings.Contains(out.String(), "CNY ") {
		t.Fatalf("output=%s json mode must not format money", out.String())
	}
}

func TestMarketListTextShowsFormattedFee(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{
			"id":"listing-1",
			"displayName":"Writer",
			"taskFixedFeeT":5000000,
			"taskFixedFee":{"amount":"0.5000000","currency":"CNY"},
			"currency":"CNY",
			"executionAvailabilityStatus":"available",
			"listingVersionId":"lv-1"
		}]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market list command error = %v", err)
	}
	assertContainsAll(t, out.String(), "CNY 0.5")
	assertContainsNone(t, out.String(), "task_fixed_fee_t")
}

func TestMarketListTextUnknownCurrencyFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"listing-1","displayName":"Writer","taskFixedFeeT":5000000}]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market list command error = %v", err)
	}
	for _, want := range []string{"(currency unknown) 5000000", "5000000"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
}

func TestMarketRunExecutionTextShowsFormattedAmounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			_, _ = w.Write([]byte(`{"quoteId":"quote-1","currency":"CNY","estimatedBuyerPayableT":5069300}`))
		case "/loom/v1/marketListings/listing-1:execute":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"runId":"run-1",
				"runTransactionId":"transaction-1",
				"skillName":"",
				"taskFixedFeeT":5000000,
				"estimatedExecutionCostT":0,
				"estimatedBuyerPayableT":5069300,
				"actualExecutionCostT":0,
				"finalBuyerPayableT":0,
				"currency":"USD",
				"transactionStatus":"reserved"
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"inputRows":[{"prompt":"review this"}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketRunCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath, "--confirm"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market run command error = %v", err)
	}
	assertContainsAll(t, out.String(),
		"USD 0.5",
		"USD 0.50693",
		"currency",
	)
	assertContainsNone(t, out.String(), "estimated_payable_t", "task_fixed_fee_t")
	// An empty skillName from the backend must not leave a blank row.
	if strings.Contains(out.String(), "skillName") {
		t.Fatalf("output=%s should omit empty skillName row", out.String())
	}
}

func TestMarketRunQuotesWithoutConfirmAndDoesNotExecute(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			_, _ = w.Write([]byte(`{"quoteId":"quote-1","currency":"CNY","estimatedBuyerPayableT":10}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"inputRows":[{"prompt":"review this"}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketRunCmd(opts)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market run command error = %v", err)
	}
	if len(paths) != 2 || paths[1] != "/loom/v1/marketListings/listing-1:quote" {
		t.Fatalf("paths=%v want detail and quote only", paths)
	}
	if strings.Contains(strings.Join(paths, ","), ":execute") {
		t.Fatalf("paths=%v must not execute without confirm", paths)
	}
	if !strings.Contains(out.String(), "quoteId") || !strings.Contains(stderr.String(), "pass --confirm") {
		t.Fatalf("out=%q stderr=%q want quote and confirm hint", out.String(), stderr.String())
	}
}

func TestMarketRunQuotesThenExecutesPublicInputRows(t *testing.T) {
	var quoteBody map[string]any
	var executeBody map[string]any
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			if err := json.NewDecoder(r.Body).Decode(&quoteBody); err != nil {
				t.Fatalf("decode quote body: %v", err)
			}
			_, _ = w.Write([]byte(`{"quoteId":"quote-1","estimatedBuyerPayableT":10}`))
		case "/loom/v1/marketListings/listing-1:execute":
			if err := json.NewDecoder(r.Body).Decode(&executeBody); err != nil {
				t.Fatalf("decode execute body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"runId":"run-1",
				"runTransactionId":"transaction-1",
				"taskFixedFeeT":5000000,
				"estimatedBuyerPayableT":5069300,
				"currency":"CNY"
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{
		"clientRequestId":"req-1",
		"user_id": 1,
		"inputRows":[{"prompt":"review this"}]
	}`)
	var logs bytes.Buffer
	opts := &rootOptions{
		server:    server.URL + "/loom/v1",
		timeout:   time.Second,
		verbose:   true,
		logWriter: &logs,
	}
	cmd := newMarketRunCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath, "--confirm"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market run command error = %v", err)
	}
	wantPaths := []string{
		"/loom/v1/marketListings/listing-1",
		"/loom/v1/marketListings/listing-1:quote",
		"/loom/v1/marketListings/listing-1:execute",
	}
	if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("paths=%v want %v", paths, wantPaths)
	}
	for _, body := range []map[string]any{quoteBody, executeBody} {
		if _, ok := body["taskInputs"]; ok {
			t.Fatalf("body should not include taskInputs: %#v", body)
		}
		if _, ok := body["listingVersionId"]; ok {
			t.Fatalf("body should not include listingVersionId: %#v", body)
		}
		if _, ok := body["user_id"]; ok {
			t.Fatalf("run body should not include user_id: %#v", body)
		}
	}
	if !reflect.DeepEqual(quoteBody["inputRows"], executeBody["inputRows"]) {
		t.Fatalf("quote inputRows=%#v execute inputRows=%#v want identical rows", quoteBody["inputRows"], executeBody["inputRows"])
	}
	if executeBody["confirm"] != true {
		t.Fatalf("confirm=%v want true", executeBody["confirm"])
	}
	if executeBody["clientRequestId"] != "req-1" {
		t.Fatalf("clientRequestId=%v want req-1", executeBody["clientRequestId"])
	}
	if !strings.Contains(logs.String(), "market run: submitted listing_id=listing-1 run_id=run-1 transaction_id=transaction-1") {
		t.Fatalf("logs=%q want market run submission identifiers", logs.String())
	}
}

func TestMarketRunPrintsGeneratedClientRequestIDBeforeRequestFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			_, _ = w.Write([]byte(`{"quoteId":"quote-1"}`))
		case "/loom/v1/marketListings/listing-1:execute":
			http.Error(w, `{"error":"temporary failure"}`, http.StatusServiceUnavailable)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"inputRows":[{"prompt":"review this"}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketRunCmd(opts)
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath, "--confirm"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("market run error = nil, want request failure")
	}
	if !strings.Contains(stderr.String(), "clientRequestId: loomloom-cli-market-") {
		t.Fatalf("stderr=%q want stable generated clientRequestId before request failure", stderr.String())
	}
}

func TestMarketRunGeneratesStableClientRequestIDWhenMissing(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			_, _ = w.Write([]byte(`{"quoteId":"quote-1"}`))
		case "/loom/v1/marketListings/listing-1:execute":
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"runId":"run-1"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"inputRows":[{"prompt":"review this"}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketRunCmd(opts)
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"listing-1", "--input-file", inputPath, "--confirm"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market run command error = %v", err)
	}
	clientRequestID, ok := body["clientRequestId"].(string)
	if !ok || !strings.HasPrefix(clientRequestID, "loomloom-cli-market-") {
		t.Fatalf("clientRequestId=%#v want stable generated loomloom-cli-market-*", body["clientRequestId"])
	}
	if body["confirm"] != true {
		t.Fatalf("confirm=%v want true", body["confirm"])
	}
	if !strings.Contains(stderr.String(), "clientRequestId: "+clientRequestID) {
		t.Fatalf("stderr=%q want generated clientRequestId", stderr.String())
	}
}

func TestMarketRunStableClientRequestIDIgnoresInputFileFormatting(t *testing.T) {
	inputA := marketInputPayload{
		ListingVersionID: "lv-1",
		InputRows: []map[string]any{{
			"prompt": "review this",
		}},
	}
	inputB := marketInputPayload{
		ListingVersionID: "lv-1",
		InputRows: []map[string]any{{
			"prompt": "review this",
		}},
	}
	idA, generatedA, err := effectiveMarketClientRequestID("", "", "listing-1", inputA)
	if err != nil {
		t.Fatalf("generate id A: %v", err)
	}
	idB, generatedB, err := effectiveMarketClientRequestID("", "", "listing-1", inputB)
	if err != nil {
		t.Fatalf("generate id B: %v", err)
	}
	if !generatedA || !generatedB || idA == "" || idA != idB {
		t.Fatalf("ids idA=%q generatedA=%t idB=%q generatedB=%t want same stable generated id", idA, generatedA, idB, generatedB)
	}
}

func TestMarketRunStableClientRequestIDIgnoresListingVersion(t *testing.T) {
	inputA := marketInputPayload{
		ListingVersionID: "lv-1",
		InputRows: []map[string]any{{
			"prompt": "review this",
		}},
	}
	inputB := marketInputPayload{
		ListingVersionID: "lv-2",
		InputRows: []map[string]any{{
			"prompt": "review this",
		}},
	}
	idA, _, err := effectiveMarketClientRequestID("", "", "listing-1", inputA)
	if err != nil {
		t.Fatalf("generate id A: %v", err)
	}
	idB, _, err := effectiveMarketClientRequestID("", "", "listing-1", inputB)
	if err != nil {
		t.Fatalf("generate id B: %v", err)
	}
	if idA == "" || idA != idB {
		t.Fatalf("ids idA=%q idB=%q want same stable id across listing versions", idA, idB)
	}
}

func TestMarketRunClientRequestIDFlagOverridesInputFile(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1":
			_, _ = w.Write([]byte(marketListingDetailBody(t)))
		case "/loom/v1/marketListings/listing-1:quote":
			_, _ = w.Write([]byte(`{"quoteId":"quote-1"}`))
		case "/loom/v1/marketListings/listing-1:execute":
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"runId":"run-1"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputPath := writeMarketInputFile(t, `{"clientRequestId":"from-file","inputRows":[{"prompt":"review this"}]}`)
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketRunCmd(opts)
	cmd.SetArgs([]string{
		"listing-1",
		"--input-file", inputPath,
		"--client-request-id", "from-flag",
		"--confirm",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market run command error = %v", err)
	}
	if body["clientRequestId"] != "from-flag" {
		t.Fatalf("clientRequestId=%v want from-flag", body["clientRequestId"])
	}
}

func TestMarketWorkbookDownloadSavesTemplate(t *testing.T) {
	var requestedPath string
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="market-template.xlsx"`)
		_, _ = w.Write([]byte("xlsx template"))
	}))
	defer server.Close()

	outDir := t.TempDir()
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newMarketWorkbookDownloadCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1", "--output-file", outDir, "--listing-version-id", "lv-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market workbook download command error = %v", err)
	}
	if requestedPath != "/loom/v1/marketListings/listing-1/workbook" {
		t.Fatalf("path=%q want workbook download endpoint", requestedPath)
	}
	if requestedQuery != "listingVersionId=lv-1" {
		t.Fatalf("query=%q want listingVersionId", requestedQuery)
	}
	target := filepath.Join(outDir, "market-template.xlsx")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read workbook: %v", err)
	}
	if string(data) != "xlsx template" {
		t.Fatalf("workbook bytes=%q", string(data))
	}
	if !strings.Contains(out.String(), `"path": "`+target+`"`) {
		t.Fatalf("output=%s want saved path", out.String())
	}
}

func TestMarketWorkbookValidateSendsWorkbookPayload(t *testing.T) {
	var request struct {
		Filename string `json:"filename"`
		Content  []byte `json:"content"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/marketListings/listing-1:validateWorkbook" {
			t.Fatalf("path=%q want validate workbook endpoint", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode workbook request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"valid":true}`))
	}))
	defer server.Close()

	workbookPath := writeMarketWorkbookFile(t, "input.xlsx", []byte("xlsx bytes"))
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newMarketWorkbookValidateCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--file", workbookPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market workbook validate command error = %v", err)
	}
	if request.Filename != "input.xlsx" {
		t.Fatalf("filename=%q want input.xlsx", request.Filename)
	}
	if string(request.Content) != "xlsx bytes" {
		t.Fatalf("content=%q want workbook bytes", string(request.Content))
	}
}

func TestMarketWorkbookRunQuotesThenExecutes(t *testing.T) {
	var quoteRequest struct {
		Filename string `json:"filename"`
		Content  []byte `json:"content"`
	}
	var executeRequest struct {
		Filename        string `json:"filename"`
		Content         []byte `json:"content"`
		Confirm         bool   `json:"confirm"`
		ClientRequestID string `json:"clientRequestId"`
	}
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/marketListings/listing-1:quoteWorkbook":
			if err := json.NewDecoder(r.Body).Decode(&quoteRequest); err != nil {
				t.Fatalf("decode quote workbook request: %v", err)
			}
			_, _ = w.Write([]byte(`{"quoteId":"quote-1","estimatedBuyerPayableT":10}`))
		case "/loom/v1/marketListings/listing-1:executeWorkbook":
			if err := json.NewDecoder(r.Body).Decode(&executeRequest); err != nil {
				t.Fatalf("decode execute workbook request: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"runId":"run-1",
				"runTransactionId":"transaction-1",
				"taskFixedFeeT":5000000,
				"estimatedBuyerPayableT":5069300,
				"currency":"CNY"
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	workbookPath := writeMarketWorkbookFile(t, "input.xlsx", []byte("xlsx bytes"))
	var logs bytes.Buffer
	opts := &rootOptions{
		server:    server.URL + "/loom/v1",
		timeout:   time.Second,
		verbose:   true,
		logWriter: &logs,
	}
	cmd := newMarketWorkbookRunCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1", "--file", workbookPath, "--client-request-id", "req-1", "--confirm"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("market workbook run command error = %v", err)
	}
	wantPaths := []string{
		"/loom/v1/marketListings/listing-1:quoteWorkbook",
		"/loom/v1/marketListings/listing-1:executeWorkbook",
	}
	if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("paths=%v want %v", paths, wantPaths)
	}
	if string(quoteRequest.Content) != "xlsx bytes" || string(executeRequest.Content) != "xlsx bytes" {
		t.Fatalf("workbook content mismatch quote=%q execute=%q", string(quoteRequest.Content), string(executeRequest.Content))
	}
	if !executeRequest.Confirm || executeRequest.ClientRequestID != "req-1" {
		t.Fatalf("execute request=%#v want confirm and clientRequestId", executeRequest)
	}
	assertContainsAll(t, out.String(), "CNY 0.5", "CNY 0.50693", "currency")
	assertContainsNone(t, out.String(), "task_fixed_fee_t", "estimated_payable_t")
	if strings.Contains(logs.String(), "eGxzeCBieXRlcw==") {
		t.Fatalf("logs should not contain workbook base64: %s", logs.String())
	}
}

func TestMarketRelistUsesListEndpointWithoutCreatorUserID(t *testing.T) {
	var requestedPath string
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"listing-1"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingRelistCmd(opts)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing relist command error = %v", err)
	}
	if requestedPath != "/loom/v1/marketListings/listing-1:list" {
		t.Fatalf("path=%q want relist endpoint", requestedPath)
	}
	if requestedQuery != "" {
		t.Fatalf("query=%q want no identity query", requestedQuery)
	}
}

func writeMarketInputFile(t *testing.T, content string) string {
	t.Helper()
	filePath := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write input file: %v", err)
	}
	return filePath
}

func writeMarketWorkbookFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	filePath := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("write workbook file: %v", err)
	}
	return filePath
}

func assertContainsAll(t *testing.T, output string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("output=%s missing %q", output, want)
		}
	}
}

func assertContainsNone(t *testing.T, output string, forbidden ...string) {
	t.Helper()
	for _, item := range forbidden {
		if strings.Contains(output, item) {
			t.Fatalf("output=%s should not contain %q", output, item)
		}
	}
}

func marketListingDetailBody(t *testing.T) string {
	t.Helper()
	schema := `{
		"schema_version": "loom_market_public_input_schema_v1",
		"fields": [
			{
				"key": "prompt",
				"label": "Prompt",
				"description": "Text prompt",
				"required": true,
				"value_type": "string",
				"source_kind": "inline_text"
			}
		],
		"instructions": ["One row per task."],
		"sample_rows": [{"prompt": "write a short note"}]
	}`
	body, err := json.Marshal(map[string]any{
		"id":                          "listing-1",
		"displayName":                 "Writer",
		"description":                 "Writes text",
		"listingVersionId":            "lv-1",
		"taskFixedFeeT":               10,
		"currency":                    "CNY",
		"executionAvailabilityStatus": "available",
		"inputSchemaSnapshot":         schema,
	})
	if err != nil {
		t.Fatalf("marshal listing detail: %v", err)
	}
	return string(body)
}
