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
)

type warningPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func jsonStringSliceEqual(value any, want []string) bool {
	items, ok := value.([]any)
	if !ok || len(items) != len(want) {
		return false
	}
	for index, item := range items {
		if item != want[index] {
			return false
		}
	}
	return true
}

func TestSkillInstallMarketDryRunOutputsPreviewAndDoesNotWrite(t *testing.T) {
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		if r.URL.Path != "/loom/v1/marketListings/listing-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"id":"listing-1",
			"displayName":"PRD Review",
			"description":"Review PRDs",
			"status":"published",
			"listingVersionId":"lv-1",
			"pricingRuleVersion":"creator_task_fixed_fee_v1",
			"taskFixedFeeT":1000000,
			"saleStatus":"listed",
			"executionAvailabilityStatus":"available",
			"inputSchemaSnapshot":"{\"fields\":[{\"key\":\"stage\",\"label\":\"Funding stage\",\"required\":true,\"value_type\":\"enum\",\"enum_values\":[\"Seed\",\"Series A\"]}]}"
		}`))
	}))
	defer server.Close()

	out := bytes.Buffer{}
	cmd := newRootCmdWithVerifiedServer(t, server.URL+"/loom/v1")
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	outputDir := filepath.Join(t.TempDir(), "loomloom-prd-review")
	cmd.SetArgs([]string{
		"--server", server.URL + "/loom/v1",
		"--output", "json",
		"skill", "install", "market", "listing-1",
		"--agent", "codex",
		"--output-dir", outputDir,
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill install market dry-run error = %v", err)
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("dry-run created output dir, stat err=%v", err)
	}
	if len(requestedPaths) != 1 || requestedPaths[0] != "/loom/v1/marketListings/listing-1" {
		t.Fatalf("requested paths=%v", requestedPaths)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if payload["previewSchemaVersion"] != "loomloom-skill-preview/v1" {
		t.Fatalf("previewSchemaVersion=%v", payload["previewSchemaVersion"])
	}
	if payload["installable"] != true {
		t.Fatalf("installable=%v", payload["installable"])
	}
	if payload["skillName"] != "loomloom-prd-review" {
		t.Fatalf("skillName=%v", payload["skillName"])
	}
	fields, ok := payload["fields"].([]any)
	if !ok || len(fields) != 1 {
		t.Fatalf("fields=%#v", payload["fields"])
	}
	field, ok := fields[0].(map[string]any)
	if !ok {
		t.Fatalf("field=%#v", fields[0])
	}
	if field["valueType"] != "enum" || !jsonStringSliceEqual(field["enumValues"], []string{"Seed", "Series A"}) {
		t.Fatalf("field=%#v", field)
	}
}

func TestSkillInstallMarketTextPreviewShowsEnumValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id":"listing-1",
			"displayName":"PRD Review",
			"status":"published",
			"listingVersionId":"lv-1",
			"saleStatus":"listed",
			"executionAvailabilityStatus":"available",
			"inputSchemaSnapshot":"{\"fields\":[{\"key\":\"stage\",\"label\":\"Funding stage\",\"required\":true,\"value_type\":\"enum\",\"enum_values\":[\"Seed\",\"Series A\"]}]}"
		}`))
	}))
	defer server.Close()

	out := bytes.Buffer{}
	cmd := newRootCmdWithVerifiedServer(t, server.URL+"/loom/v1")
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--server", server.URL + "/loom/v1",
		"skill", "install", "market", "listing-1",
		"--agent", "codex",
		"--output-dir", filepath.Join(t.TempDir(), "loomloom-prd-review"),
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill install market text dry-run error = %v", err)
	}
	for _, want := range []string{"fields:", "stage", "Funding stage", `["Seed","Series A"]`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestSkillInstallMarketWritesConcreteListingID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/marketListings/listing-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"id":"listing-1",
			"displayName":"PRD Review",
			"status":"published",
			"listingVersionId":"lv-1",
			"saleStatus":"listed",
			"executionAvailabilityStatus":"available",
			"inputSchemaSnapshot":"{\"fields\":[{\"key\":\"stage\",\"label\":\"Funding stage\",\"required\":true,\"value_type\":\"enum\",\"enum_values\":[\"Seed\",\"Series A\"],\"future_field\":{\"kept\":true}}],\"future_top_level\":\"kept\"}"
		}`))
	}))
	defer server.Close()

	out := bytes.Buffer{}
	cmd := newRootCmdWithVerifiedServer(t, server.URL+"/loom/v1")
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	outputDir := filepath.Join(t.TempDir(), "loomloom-prd-review")
	cmd.SetArgs([]string{
		"--server", server.URL + "/loom/v1",
		"skill", "install", "market", "listing-1",
		"--agent", "codex",
		"--output-dir", outputDir,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill install market error = %v", err)
	}
	skillBytes, err := os.ReadFile(filepath.Join(outputDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	skillText := string(skillBytes)
	if !strings.Contains(skillText, "loomloom market show listing-1") ||
		!strings.Contains(skillText, "loomloom market run listing-1") {
		t.Fatalf("SKILL.md did not include concrete listing ID:\n%s", skillText)
	}
	if strings.Contains(skillText, "<listing-id>") {
		t.Fatalf("SKILL.md still contains listing placeholder:\n%s", skillText)
	}
	if !strings.Contains(skillText, "Allowed values (server data):") || !strings.Contains(skillText, "Seed") || !strings.Contains(skillText, "Series A") {
		t.Fatalf("SKILL.md missing enum values:\n%s", skillText)
	}
	metadataBytes, err := os.ReadFile(filepath.Join(outputDir, "loomloom-skill.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if !strings.Contains(string(metadataBytes), "future_field") || !strings.Contains(string(metadataBytes), "future_top_level") {
		t.Fatalf("metadata did not preserve raw snapshot: %s", metadataBytes)
	}
}

func TestSkillInstallTemplateSpecWritesFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loom/v1/users/me/templates/tmpl-1":
			_, _ = w.Write([]byte(`{"templateId":"tmpl-1","name":"Internal Report","description":"Prepare: reports"}`))
		case "/loom/v1/users/me/templates/tmpl-1/versions":
			_, _ = w.Write([]byte(`{"items":[{"versionId":"ver-1","inputSchemaSnapshot":{"fields":[{"key":"topic","label":"Topic","required":true,"value_type":"text"}],"instructions":["Keep it concise"],"sample_rows":[{"topic":"Q3"}]}}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	out := bytes.Buffer{}
	cmd := newRootCmdWithVerifiedServer(t, server.URL+"/loom/v1")
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	outputDir := filepath.Join(t.TempDir(), "loomloom-internal-report")
	cmd.SetArgs([]string{
		"--server", server.URL + "/loom/v1",
		"skill", "install", "template-spec", "tmpl-1", "ver-1",
		"--agent", "claude",
		"--output-dir", outputDir,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill install template-spec error = %v", err)
	}
	skillBytes, err := os.ReadFile(filepath.Join(outputDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	if !strings.Contains(string(skillBytes), "template-spec submit-workbook tmpl-1 ver-1") {
		t.Fatalf("SKILL.md missing private workbook flow:\n%s", string(skillBytes))
	}
	if !strings.Contains(string(skillBytes), `description: "Prepare: reports"`) {
		t.Fatalf("SKILL.md frontmatter description was not quoted:\n%s", string(skillBytes))
	}
	if !strings.Contains(string(skillBytes), "Keep it concise") || !strings.Contains(string(skillBytes), `{"topic":"Q3"}`) {
		t.Fatalf("SKILL.md missing schema instructions or sample rows:\n%s", string(skillBytes))
	}
	if !strings.Contains(string(skillBytes), "loomloom input-asset upload <file>") ||
		!strings.Contains(string(skillBytes), "input_asset_id") ||
		!strings.Contains(string(skillBytes), "Do not guess, invent, or substitute important user materials") ||
		!strings.Contains(string(skillBytes), "Do not use them as the user's actual input unless the user explicitly confirms those values") ||
		!strings.Contains(string(skillBytes), "Do not continue to validate, quote/precheck, or submit with guessed placeholder content") {
		t.Fatalf("SKILL.md missing file input upload guidance:\n%s", string(skillBytes))
	}
	if !strings.Contains(string(skillBytes), "translate technical field names and enum values into plain user-facing wording") ||
		!strings.Contains(string(skillBytes), "This SkillBot has been forcibly removed from the Market and cannot currently be listed or run") ||
		!strings.Contains(string(skillBytes), "the review was not approved") ||
		!strings.Contains(string(skillBytes), "currently unavailable to run") {
		t.Fatalf("SKILL.md missing user-facing status wording guidance:\n%s", string(skillBytes))
	}
	metadataBytes, err := os.ReadFile(filepath.Join(outputDir, "loomloom-skill.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if strings.Contains(string(metadataBytes), "Authorization") || strings.Contains(string(metadataBytes), "Bearer") {
		t.Fatalf("metadata leaked auth data: %s", string(metadataBytes))
	}
	var metadata map[string]any
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata["source_type"] != "user_template" {
		t.Fatalf("source_type=%v", metadata["source_type"])
	}
	if metadata["source_id"] != "tmpl-1:ver-1" {
		t.Fatalf("source_id=%v", metadata["source_id"])
	}
	if metadata["skill_name"] != "loomloom-internal-report" {
		t.Fatalf("skill_name=%v", metadata["skill_name"])
	}
}

func TestSkillInstallTemplateSpecUsesRequestedVersionSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loom/v1/users/me/templates/tmpl-1":
			_, _ = w.Write([]byte(`{"templateId":"tmpl-1","name":"Versioned Report"}`))
		case "/loom/v1/users/me/templates/tmpl-1/versions":
			_, _ = w.Write([]byte(`{"items":[
				{"versionId":"ver-old","inputSchemaSnapshot":{"fields":[{"key":"old_topic","label":"Old Topic","required":true}]}},
				{"versionId":"ver-1","inputSchemaSnapshot":{"fields":[{"key":"new_topic","label":"New Topic","required":true}],"instructions":["Use the new schema"]}}
			]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	out := bytes.Buffer{}
	cmd := newRootCmdWithVerifiedServer(t, server.URL+"/loom/v1")
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	outputDir := filepath.Join(t.TempDir(), "loomloom-versioned-report")
	cmd.SetArgs([]string{
		"--server", server.URL + "/loom/v1",
		"skill", "install", "template-spec", "tmpl-1", "ver-1",
		"--agent", "codex",
		"--output-dir", outputDir,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill install template-spec error = %v", err)
	}
	skillBytes, err := os.ReadFile(filepath.Join(outputDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	skillText := string(skillBytes)
	if !strings.Contains(skillText, "new_topic") || !strings.Contains(skillText, "Use the new schema") {
		t.Fatalf("SKILL.md did not use requested version schema:\n%s", skillText)
	}
	if strings.Contains(skillText, "old_topic") {
		t.Fatalf("SKILL.md used old version schema:\n%s", skillText)
	}
}

func TestSkillInstallTemplateSpecRequiresVersionObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loom/v1/users/me/templates/tmpl-1":
			_, _ = w.Write([]byte(`{"templateId":"tmpl-1","name":"Versioned Report"}`))
		case "/loom/v1/users/me/templates/tmpl-1/versions":
			_, _ = w.Write([]byte(`{"items":[
				{"versionId":"ver-old","notes":"mentions ver-1 but is not that version","inputSchemaSnapshot":{"fields":[{"key":"old_topic","label":"Old Topic","required":true}]}}
			]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	out := bytes.Buffer{}
	cmd := newRootCmdWithVerifiedServer(t, server.URL+"/loom/v1")
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	outputDir := filepath.Join(t.TempDir(), "loomloom-versioned-report")
	cmd.SetArgs([]string{
		"--server", server.URL + "/loom/v1",
		"skill", "install", "template-spec", "tmpl-1", "ver-1",
		"--agent", "codex",
		"--output-dir", outputDir,
	})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "template version ver-1 was not found") {
		t.Fatalf("error=%v, want missing requested version", err)
	}
}

func TestSkillInstallTemplateSpecPrefersDirectVersionSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loom/v1/users/me/templates/tmpl-1":
			_, _ = w.Write([]byte(`{"templateId":"tmpl-1","name":"Versioned Report"}`))
		case "/loom/v1/users/me/templates/tmpl-1/versions":
			_, _ = w.Write([]byte(`{"items":[{
				"versionId":"ver-1",
				"inputSchemaSnapshot":{"fields":[{"key":"direct_topic","label":"Direct Topic","required":true}]},
				"history":[{"versionId":"ver-old","inputSchemaSnapshot":{"fields":[{"key":"old_topic","label":"Old Topic","required":true}]}}]
			}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	out := bytes.Buffer{}
	cmd := newRootCmdWithVerifiedServer(t, server.URL+"/loom/v1")
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	outputDir := filepath.Join(t.TempDir(), "loomloom-versioned-report")
	cmd.SetArgs([]string{
		"--server", server.URL + "/loom/v1",
		"skill", "install", "template-spec", "tmpl-1", "ver-1",
		"--agent", "codex",
		"--output-dir", outputDir,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill install template-spec error = %v", err)
	}
	skillBytes, err := os.ReadFile(filepath.Join(outputDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	skillText := string(skillBytes)
	if !strings.Contains(skillText, "direct_topic") {
		t.Fatalf("SKILL.md did not use direct schema:\n%s", skillText)
	}
	if strings.Contains(skillText, "old_topic") {
		t.Fatalf("SKILL.md used nested old schema:\n%s", skillText)
	}
}

func TestSkillInstallTemplateSpecWorkbookOnlyDryRunWarns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loom/v1/users/me/templates/tmpl-1":
			_, _ = w.Write([]byte(`{"templateId":"tmpl-1","name":"Workbook Only"}`))
		case "/loom/v1/users/me/templates/tmpl-1/versions":
			_, _ = w.Write([]byte(`{"items":[{"versionId":"ver-1"}]}`))
		case "/loom/v1/users/me/executables":
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	out := bytes.Buffer{}
	cmd := newRootCmdWithVerifiedServer(t, server.URL+"/loom/v1")
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	outputDir := filepath.Join(t.TempDir(), "loomloom-workbook-only")
	cmd.SetArgs([]string{
		"--server", server.URL + "/loom/v1",
		"--output", "json",
		"skill", "install", "template-spec", "tmpl-1", "ver-1",
		"--agent", "codex",
		"--output-dir", outputDir,
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill install template-spec dry-run error = %v", err)
	}
	var payload struct {
		InputSchemaMode string           `json:"inputSchemaMode"`
		Warnings        []warningPayload `json:"warnings"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if payload.InputSchemaMode != "workbook_only" {
		t.Fatalf("inputSchemaMode=%q", payload.InputSchemaMode)
	}
	if !hasWarningCode(payload.Warnings, "input_schema_workbook_only") {
		t.Fatalf("warnings=%#v, want input_schema_workbook_only", payload.Warnings)
	}
}

func TestSkillInstallTemplateSpecWorkbookOnlyInstallWarns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loom/v1/users/me/templates/tmpl-1":
			_, _ = w.Write([]byte(`{"templateId":"tmpl-1","name":"Workbook Only"}`))
		case "/loom/v1/users/me/templates/tmpl-1/versions":
			_, _ = w.Write([]byte(`{"items":[{"versionId":"ver-1"}]}`))
		case "/loom/v1/users/me/executables":
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	out := bytes.Buffer{}
	cmd := newRootCmdWithVerifiedServer(t, server.URL+"/loom/v1")
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	outputDir := filepath.Join(t.TempDir(), "loomloom-workbook-only")
	cmd.SetArgs([]string{
		"--server", server.URL + "/loom/v1",
		"skill", "install", "template-spec", "tmpl-1", "ver-1",
		"--agent", "codex",
		"--output-dir", outputDir,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill install template-spec error = %v", err)
	}
	if !strings.Contains(out.String(), "warning\tinput_schema_workbook_only") {
		t.Fatalf("install output missing workbook_only warning:\n%s", out.String())
	}
	metadataBytes, err := os.ReadFile(filepath.Join(outputDir, "loomloom-skill.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var metadata struct {
		InputSchemaMode string `json:"input_schema_mode"`
	}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatalf("decode metadata: %v\n%s", err, string(metadataBytes))
	}
	if metadata.InputSchemaMode != "workbook_only" {
		t.Fatalf("input_schema_mode=%q, want workbook_only", metadata.InputSchemaMode)
	}
	skillBytes, err := os.ReadFile(filepath.Join(outputDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	if !strings.Contains(string(skillBytes), "Full structured input schema was not available") {
		t.Fatalf("SKILL.md missing workbook_only guidance:\n%s", string(skillBytes))
	}
}

func hasWarningCode(warnings []warningPayload, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func TestSkillInstallDryRunConflictReturnsStructuredError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id":"listing-1",
			"displayName":"PRD Review",
			"status":"published",
			"listingVersionId":"lv-1",
			"saleStatus":"listed",
			"executionAvailabilityStatus":"available",
			"inputSchemaSnapshot":"{\"fields\":[{\"key\":\"prompt\",\"label\":\"Prompt\",\"required\":true}]}"
		}`))
	}))
	defer server.Close()

	outputDir := filepath.Join(t.TempDir(), "loomloom-prd-review")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := bytes.Buffer{}
	cmd := newRootCmdWithVerifiedServer(t, server.URL+"/loom/v1")
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--server", server.URL + "/loom/v1",
		"--output", "json",
		"skill", "install", "market", "listing-1",
		"--agent", "codex",
		"--output-dir", outputDir,
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run conflict should return structured preview, got err=%v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload["installable"] != false {
		t.Fatalf("installable=%v", payload["installable"])
	}
	if payload["skillNameConflict"] != true {
		t.Fatalf("skillNameConflict=%v", payload["skillNameConflict"])
	}
	if payload["blockingReason"] != "skill_name_conflict" {
		t.Fatalf("blockingReason=%v", payload["blockingReason"])
	}
}

func TestSkillInstallDryRunMissingParentIsOutputDirError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id":"listing-1",
			"displayName":"PRD Review",
			"status":"published",
			"listingVersionId":"lv-1",
			"saleStatus":"listed",
			"executionAvailabilityStatus":"available",
			"inputSchemaSnapshot":"{\"fields\":[{\"key\":\"prompt\",\"label\":\"Prompt\",\"required\":true}]}"
		}`))
	}))
	defer server.Close()

	out := bytes.Buffer{}
	cmd := newRootCmdWithVerifiedServer(t, server.URL+"/loom/v1")
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	outputDir := filepath.Join(t.TempDir(), "missing-parent", "loomloom-prd-review")
	cmd.SetArgs([]string{
		"--server", server.URL + "/loom/v1",
		"--output", "json",
		"skill", "install", "market", "listing-1",
		"--agent", "codex",
		"--output-dir", outputDir,
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run output dir error should return structured preview, got err=%v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload["installable"] != false {
		t.Fatalf("installable=%v", payload["installable"])
	}
	if payload["skillNameConflict"] != false {
		t.Fatalf("skillNameConflict=%v", payload["skillNameConflict"])
	}
	if payload["blockingReason"] != "output_dir_unavailable" {
		t.Fatalf("blockingReason=%v", payload["blockingReason"])
	}
}

func TestSkillInstallDryRunOutputDirMustMatchGeneratedSkillName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id":"listing-1",
			"displayName":"PRD Review",
			"status":"published",
			"listingVersionId":"lv-1",
			"saleStatus":"listed",
			"executionAvailabilityStatus":"available",
			"inputSchemaSnapshot":"{\"fields\":[{\"key\":\"prompt\",\"label\":\"Prompt\",\"required\":true}]}"
		}`))
	}))
	defer server.Close()

	out := bytes.Buffer{}
	cmd := newRootCmdWithVerifiedServer(t, server.URL+"/loom/v1")
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	outputDir := filepath.Join(t.TempDir(), "prd-review")
	cmd.SetArgs([]string{
		"--server", server.URL + "/loom/v1",
		"--output", "json",
		"skill", "install", "market", "listing-1",
		"--agent", "codex",
		"--output-dir", outputDir,
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run name mismatch should return structured preview, got err=%v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload["installable"] != false {
		t.Fatalf("installable=%v", payload["installable"])
	}
	if payload["skillName"] != "loomloom-prd-review" {
		t.Fatalf("skillName=%v", payload["skillName"])
	}
	if payload["blockingReason"] != "output_dir_name_mismatch" {
		t.Fatalf("blockingReason=%v", payload["blockingReason"])
	}
}

func TestSkillUninstallDryRunOutputsPreviewAndDoesNotDelete(t *testing.T) {
	skillDir := writeInstalledSkillDir(t, false)

	out := bytes.Buffer{}
	cmd := NewRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--output", "json",
		"skill", "uninstall",
		"--dir", skillDir,
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill uninstall dry-run error = %v", err)
	}
	if _, err := os.Stat(skillDir); err != nil {
		t.Fatalf("dry-run removed skill dir: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if payload["previewSchemaVersion"] != "loomloom-skill-uninstall-preview/v1" {
		t.Fatalf("previewSchemaVersion=%v", payload["previewSchemaVersion"])
	}
	if payload["removable"] != true {
		t.Fatalf("removable=%v", payload["removable"])
	}
	if payload["skillName"] != "test-skill" {
		t.Fatalf("skillName=%v", payload["skillName"])
	}
}

func TestSkillUninstallRemovesDirectory(t *testing.T) {
	skillDir := writeInstalledSkillDir(t, false)

	out := bytes.Buffer{}
	cmd := NewRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"skill", "uninstall",
		"--dir", skillDir,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill uninstall error = %v", err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatalf("skill dir still exists, stat err=%v", err)
	}
	if !strings.Contains(out.String(), "uninstalled\ttrue") ||
		!strings.Contains(out.String(), "skill_name\ttest-skill") {
		t.Fatalf("unexpected uninstall output:\n%s", out.String())
	}
}

func TestSkillUninstallMissingMetadataDryRunBlocks(t *testing.T) {
	skillDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := bytes.Buffer{}
	cmd := NewRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--output", "json",
		"skill", "uninstall",
		"--dir", skillDir,
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run should return structured preview, got err=%v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if payload["removable"] != false {
		t.Fatalf("removable=%v", payload["removable"])
	}
	if payload["blockingReason"] != "metadata_missing" {
		t.Fatalf("blockingReason=%v", payload["blockingReason"])
	}
}

func TestSkillUninstallMissingDirDryRunOutputsStructuredPreview(t *testing.T) {
	out := bytes.Buffer{}
	cmd := NewRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--output", "json",
		"skill", "uninstall",
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run should return structured preview, got err=%v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if payload["removable"] != false {
		t.Fatalf("removable=%v", payload["removable"])
	}
	if payload["blockingReason"] != "dir_required" {
		t.Fatalf("blockingReason=%v", payload["blockingReason"])
	}
}

func TestSkillUninstallUnexpectedFilesRequireForce(t *testing.T) {
	skillDir := writeInstalledSkillDir(t, true)

	out := bytes.Buffer{}
	cmd := NewRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--output", "json",
		"skill", "uninstall",
		"--dir", skillDir,
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run should return structured preview, got err=%v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if payload["removable"] != false {
		t.Fatalf("removable=%v", payload["removable"])
	}
	if payload["blockingReason"] != "unexpected_files" {
		t.Fatalf("blockingReason=%v", payload["blockingReason"])
	}
	if _, err := os.Stat(skillDir); err != nil {
		t.Fatalf("dry-run removed skill dir: %v", err)
	}
}

func TestSkillUninstallForceRemovesUnexpectedFiles(t *testing.T) {
	skillDir := writeInstalledSkillDir(t, true)

	out := bytes.Buffer{}
	cmd := NewRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--output", "json",
		"skill", "uninstall",
		"--dir", skillDir,
		"--force",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill uninstall --force error = %v", err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatalf("skill dir still exists, stat err=%v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if payload["uninstalled"] != true {
		t.Fatalf("uninstalled=%v", payload["uninstalled"])
	}
}

func writeInstalledSkillDir(t *testing.T, extraFile bool) string {
	t.Helper()
	skillDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	metadata := `{
		"schema_version":"loomloom-skill/v1",
		"generated_by":"loomloom-cli",
		"source_type":"market_listing",
		"agent":"codex",
		"generated_at":"2026-06-30T00:00:00Z",
		"skill_name":"test-skill",
		"display_name":"Test Skill",
		"source_id":"listing-1",
		"input_schema_mode":"schema",
		"listing_id":"listing-1"
	}`
	if err := os.WriteFile(filepath.Join(skillDir, "loomloom-skill.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	if extraFile {
		if err := os.WriteFile(filepath.Join(skillDir, "notes.txt"), []byte("keep me"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return skillDir
}
