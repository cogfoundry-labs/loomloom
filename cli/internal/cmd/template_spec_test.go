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
	"unicode"

	"github.com/cogfoundry-labs/loomloom/cli/internal/platform"
	templatespecdocs "github.com/cogfoundry-labs/loomloom/cli/internal/template_spec_docs"
	"github.com/spf13/cobra"
)

func TestGeneratedTemplateSpecExamplesAreEnglish(t *testing.T) {
	root := findRepoRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, "docs", "template-spec", "en", "examples", "*", "*.json"))
	if err != nil {
		t.Fatalf("glob generated examples: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("generated TemplateSpec examples are missing")
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, r := range string(data) {
			if unicode.Is(unicode.Han, r) {
				t.Fatalf("generated English example contains Chinese text: %s", path)
			}
		}
	}
}

func TestGeneratedValidTemplateSpecExamplesPassCLIValidation(t *testing.T) {
	root := findRepoRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, "docs", "template-spec", "en", "examples", "valid", "*.json"))
	if err != nil {
		t.Fatalf("glob generated valid examples: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("generated valid TemplateSpec examples are missing")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			if _, _, err := loadTemplateSpecFile(path); err != nil {
				t.Fatalf("generated valid example failed CLI validation: %v", err)
			}
		})
	}
}

func TestGeneratedExpandedCompatibilityExampleFailsNewAuthoringValidation(t *testing.T) {
	path := filepath.Join(findRepoRoot(t), "docs", "template-spec", "en", "examples", "invalid", "expanded-execution-fan-out.json")

	_, _, err := loadTemplateSpecFile(path)

	if err == nil || !strings.Contains(err.Error(), templateSpecExpandedAuthoringPolicyCode) {
		t.Fatalf("loadTemplateSpecFile() error = %v, want %s", err, templateSpecExpandedAuthoringPolicyCode)
	}
}

func requireGeneratedTemplateSpecDocs(t *testing.T) {
	t.Helper()
	if _, err := templatespecdocs.ReadManifest(); err != nil {
		t.Skip("generated TemplateSpec docs were not prepared")
	}
}

func TestLoadTemplateSpecFile_ValidSpec(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "Meta": {"Name": "Spec Test", "Description": "desc"},
  "Steps": [{"StepID": "stp_text01", "DisplayName": "Text", "ExecutionUnit": "text-generate"}],
  "InputSchema": {"Fields": [{"Key": "prompt", "Label": "Prompt", "ValueType": "string"}]},
  "FieldBindings": [{"FieldKey": "prompt", "StepID": "stp_text01", "ParamKey": "prompt", "BindMode": "shared"}]
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	spec, raw, err := loadTemplateSpecFile(path)
	if err != nil {
		t.Fatalf("loadTemplateSpecFile() error = %v", err)
	}
	if spec.Meta.Name != "Spec Test" {
		t.Fatalf("Meta.Name = %q, want Spec Test", spec.Meta.Name)
	}
	if len(raw) == 0 || raw[0] != '{' {
		t.Fatalf("expected compact JSON bytes, got %q", string(raw))
	}
	if !strings.Contains(string(raw), `"meta"`) {
		t.Fatalf("expected normalized lowerCamel TemplateSpec JSON, got %s", string(raw))
	}
	if strings.Contains(string(raw), `"Meta"`) {
		t.Fatalf("expected PascalCase keys to be normalized, got %s", string(raw))
	}
}

func TestLoadTemplateSpecFile_RejectsExpandedForNewAuthoring(t *testing.T) {
	tests := []struct {
		name     string
		bindings string
	}{
		{name: "field binding", bindings: `"fieldBindings":[{"fieldKey":"prompts","stepId":"stp_text01","paramKey":"prompt","bindMode":"expanded"}]`},
		{name: "param binding", bindings: `"paramBindings":[{"stepId":"stp_text01","paramKey":"prompt","bindMode":"expanded","sources":[{"kind":"field_ref","fieldKey":"prompts"}]}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "spec.json")
			content := `{
  "meta":{"name":"Legacy expansion"},
  "steps":[{"stepId":"stp_text01","executionUnit":"text-generate"}],
  "inputSchema":{"fields":[{"key":"prompts","label":"Prompts","valueType":"string","multiValue":true,"maxValues":10}]},
  ` + tt.bindings + `
}`
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatalf("write spec: %v", err)
			}

			_, _, err := loadTemplateSpecFile(path)

			if err == nil || !strings.Contains(err.Error(), templateSpecExpandedAuthoringPolicyCode) {
				t.Fatalf("loadTemplateSpecFile() error = %v, want %s", err, templateSpecExpandedAuthoringPolicyCode)
			}
		})
	}
}

func TestLoadTemplateSpecFile_AllowsMultiValueInitialInputCollection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "meta":{"name":"Reference summary"},
  "steps":[{
    "stepId":"stp_text01",
    "executionUnit":"text-generate",
    "upstreamBindings":[{"inputPort":"reference","sourceType":"initial_input","sourceInputKey":"references"}]
  }],
  "inputSchema":{"fields":[{"key":"references","label":"References","valueType":"text_reference","acceptedMimeTypes":["text/*"],"multiValue":true,"maxValues":10}]}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	if _, _, err := loadTemplateSpecFile(path); err != nil {
		t.Fatalf("loadTemplateSpecFile() error = %v", err)
	}
}

func TestLoadTemplateSpecFile_MissingName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "Meta": {},
  "Steps": [{"StepID": "stp_text01"}],
  "InputSchema": {"Fields": []},
  "FieldBindings": []
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	if _, _, err := loadTemplateSpecFile(path); err == nil {
		t.Fatal("loadTemplateSpecFile() error = nil, want missing name error")
	}
}

func TestLoadTemplateSpecFile_NormalizesPascalCaseUpstreamBindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "Meta": {"Name": "Initial Input Binding Spec"},
  "Steps": [{
    "StepID": "stp_text01",
    "DisplayName": "Text",
    "ExecutionUnit": "text-generate",
    "UpstreamBindings": [{
      "InputPort": "prompt",
      "SourceType": "initial_input",
      "SourceInputKey": "patent_input"
    }]
  }],
  "InputSchema": {"Fields": [{
    "Key": "patent_input",
    "Label": "Patent Input",
    "ValueType": "text_reference",
    "AcceptedMIMETypes": ["text/plain"]
  }]},
  "FieldBindings": []
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	_, raw, err := loadTemplateSpecFile(path)
	if err != nil {
		t.Fatalf("loadTemplateSpecFile() error = %v", err)
	}
	normalized := string(raw)
	if !strings.Contains(normalized, `"upstreamBindings"`) {
		t.Fatalf("normalized spec missing upstreamBindings: %s", normalized)
	}
	if strings.Contains(normalized, `"UpstreamBindings"`) {
		t.Fatalf("normalized spec still has PascalCase UpstreamBindings: %s", normalized)
	}
	if !strings.Contains(normalized, `"acceptedMimeTypes"`) {
		t.Fatalf("normalized spec missing acceptedMimeTypes: %s", normalized)
	}
}

func TestLoadTemplateSpecFile_AllowsParamBindingOnlySpec(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "meta": {"name": "Param Binding Spec"},
  "steps": [{"stepId": "stp_text01", "displayName": "Text", "executionUnit": "text-generate"}],
  "inputSchema": {"fields": [{"key": "prompt", "label": "Prompt", "valueType": "string"}]},
  "paramBindings": [{
    "stepId": "stp_text01",
    "paramKey": "prompt",
    "bindMode": "shared",
    "sources": [{"kind": "field_ref", "fieldKey": "prompt"}]
  }]
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	spec, raw, err := loadTemplateSpecFile(path)
	if err != nil {
		t.Fatalf("loadTemplateSpecFile() error = %v", err)
	}
	if len(spec.FieldBindings) != 0 {
		t.Fatalf("FieldBindings len = %d, want 0", len(spec.FieldBindings))
	}
	if len(spec.ParamBindings) != 1 {
		t.Fatalf("ParamBindings len = %d, want 1", len(spec.ParamBindings))
	}
	if !strings.Contains(string(raw), `"paramBindings"`) {
		t.Fatalf("normalized spec missing paramBindings: %s", string(raw))
	}
}

func TestLoadTemplateSpecFile_RejectsTextReferenceFieldBindingToPrompt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "meta": {"name": "Invalid Text Reference Binding"},
  "steps": [{"stepId": "stp_text01", "displayName": "Text", "executionUnit": "text-generate"}],
  "inputSchema": {"fields": [{
    "key": "patent_input",
    "label": "Patent Input",
    "valueType": "text_reference",
    "acceptedMimeTypes": ["text/plain"]
  }]},
  "fieldBindings": [{
    "fieldKey": "patent_input",
    "stepId": "stp_text01",
    "paramKey": "prompt",
    "bindMode": "shared"
  }]
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	_, _, err := loadTemplateSpecFile(path)
	if err == nil {
		t.Fatal("loadTemplateSpecFile() error = nil, want text_reference binding error")
	}
	if !strings.Contains(err.Error(), "text_reference") || !strings.Contains(err.Error(), "initial_input") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadTemplateSpecFile_AllowsThreeVisibleFieldParamBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "meta": {"name": "Three Field Prompt Spec"},
  "steps": [{"stepId": "stp_text01", "displayName": "Text", "executionUnit": "text-generate"}],
  "inputSchema": {"fields": [
    {"key": "body", "label": "Body", "valueType": "string"},
    {"key": "style", "label": "Style requirements", "valueType": "string"},
    {"key": "format", "label": "Output format", "valueType": "string"}
  ]},
  "paramBindings": [{
    "stepId": "stp_text01",
    "paramKey": "prompt",
    "bindMode": "shared",
    "separator": "\n\n",
    "sources": [
      {"kind": "field_ref", "fieldKey": "body"},
      {"kind": "field_ref", "fieldKey": "style"},
      {"kind": "field_ref", "fieldKey": "format"}
    ]
  }]
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	spec, raw, err := loadTemplateSpecFile(path)
	if err != nil {
		t.Fatalf("loadTemplateSpecFile() error = %v", err)
	}
	if len(spec.ParamBindings) != 1 {
		t.Fatalf("ParamBindings len = %d, want 1", len(spec.ParamBindings))
	}
	if !strings.Contains(string(raw), `"fieldKey":"format"`) {
		t.Fatalf("normalized spec missing third field source: %s", string(raw))
	}
}

func TestTemplateSpecCheckCmdCountsParamBindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "meta": {"name": "Param Binding Spec"},
  "steps": [{"stepId": "stp_text01", "displayName": "Text", "executionUnit": "text-generate"}],
  "inputSchema": {"fields": [{"key": "prompt", "label": "Prompt", "valueType": "string"}]},
  "paramBindings": [{
    "stepId": "stp_text01",
    "paramKey": "prompt",
    "bindMode": "shared",
    "sources": [{"kind": "field_ref", "fieldKey": "prompt"}]
  }]
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	opts := &rootOptions{output: "text"}
	cmd := newTemplateSpecCheckCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("check command error = %v", err)
	}
	if !strings.Contains(out.String(), "bindings\t1") {
		t.Fatalf("output missing binding count: %s", out.String())
	}
}

func TestTemplateSpecDocsCmdListsTopics(t *testing.T) {
	requireGeneratedTemplateSpecDocs(t)
	opts := &rootOptions{output: "text"}
	cmd := newTemplateSpecDocsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("docs command error = %v", err)
	}
	output := out.String()
	for _, want := range []string{"TemplateSpec revision: sha256:", "Owner: loomloom-docs", "spec", "authoring", "examples", "conversation", "metadata", "inputs", "steps", "bindings", "execution-units", "loomloom template-spec docs <topic>"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
}

func TestTemplateSpecDocsCmdPrintsConversation(t *testing.T) {
	requireGeneratedTemplateSpecDocs(t)
	opts := &rootOptions{output: "text"}
	cmd := newTemplateSpecDocsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"conversation"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("docs conversation command error = %v", err)
	}
	output := out.String()
	for _, want := range []string{"# Understand TemplateSpec", "immutable snapshots", "Local check is not execution"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
}

func TestTemplateSpecDocsCmdPrintsAuthoringDiscoveryPath(t *testing.T) {
	requireGeneratedTemplateSpecDocs(t)
	opts := &rootOptions{output: "text"}
	cmd := newTemplateSpecDocsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"authoring"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("docs authoring command error = %v", err)
	}
	output := out.String()
	for _, want := range []string{"# TemplateSpec quickstart", "docs spec", "docs inputs", "docs steps", "docs bindings", "docs execution-units", "do not add `branch` or `parallel`", "do not use `expanded`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
}

func TestTemplateSpecDocsCmdPrintsSpec(t *testing.T) {
	requireGeneratedTemplateSpecDocs(t)
	opts := &rootOptions{output: "text"}
	cmd := newTemplateSpecDocsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"spec"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("docs spec command error = %v", err)
	}
	output := out.String()
	for _, want := range []string{"# TemplateSpec syntax", "lowerCamel", "inputSchema", "fieldBindings", "Field quick reference", "valueType`, not `type`", "docs execution-units"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
}

func TestTemplateSpecDocsCmdPrintsChineseSpec(t *testing.T) {
	requireGeneratedTemplateSpecDocs(t)
	opts := &rootOptions{output: "text"}
	cmd := newTemplateSpecDocsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"spec", "--lang", "zh-CN"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Chinese docs command error = %v", err)
	}
	output := out.String()
	for _, want := range []string{"Language: zh-CN", "# TemplateSpec 语法参考", "顶层对象", "字段"} {
		if !strings.Contains(output, want) {
			t.Fatalf("Chinese output missing %q: %s", want, output)
		}
	}
}

func TestTemplateSpecDocsCmdRejectsUnknownLanguage(t *testing.T) {
	opts := &rootOptions{output: "text"}
	cmd := newTemplateSpecDocsCmd(opts)
	cmd.SetArgs([]string{"spec", "--lang", "fr"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "use en or zh-CN") {
		t.Fatalf("unexpected language error: %v", err)
	}
}

func TestTemplateSpecDocsCmdSupportsJSON(t *testing.T) {
	requireGeneratedTemplateSpecDocs(t)
	opts := &rootOptions{output: "json"}
	cmd := newTemplateSpecDocsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"examples"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("docs examples command error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode docs JSON: %v", err)
	}
	if payload["topic"] != "examples" {
		t.Fatalf("topic=%v want examples", payload["topic"])
	}
	if payload["language"] != "en" {
		t.Fatalf("language=%v want en", payload["language"])
	}
	if revision, _ := payload["languageRevision"].(string); !strings.HasPrefix(revision, "sha256:") {
		t.Fatalf("languageRevision=%v want sha256 revision", payload["languageRevision"])
	}
	if revision, _ := payload["specRevision"].(string); !strings.HasPrefix(revision, "sha256:") {
		t.Fatalf("specRevision=%v want sha256 revision", payload["specRevision"])
	}
	if payload["owner"] != "loomloom-docs" {
		t.Fatalf("owner=%v want loomloom-docs", payload["owner"])
	}
	content, _ := payload["content"].(string)
	for _, want := range []string{"# TemplateSpec examples", "single-text-generation", "uploaded-text-reference", "Capability", "parallel-text-to-image-branches", "no `branch` or `parallel` property", "template-spec check"} {
		if !strings.Contains(content, want) {
			t.Fatalf("content missing %q: %s", want, content)
		}
	}
}

func TestTemplateSpecDocsCmdSupportsChineseJSON(t *testing.T) {
	requireGeneratedTemplateSpecDocs(t)
	opts := &rootOptions{output: "json"}
	cmd := newTemplateSpecDocsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"examples", "--lang", "zh-CN"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Chinese docs JSON command error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode Chinese docs JSON: %v", err)
	}
	if payload["language"] != "zh-CN" {
		t.Fatalf("language=%v want zh-CN", payload["language"])
	}
	if revision, _ := payload["languageRevision"].(string); !strings.HasPrefix(revision, "sha256:") {
		t.Fatalf("languageRevision=%v want sha256 revision", payload["languageRevision"])
	}
	if content, _ := payload["content"].(string); !strings.Contains(content, "# TemplateSpec Examples") || !strings.Contains(content, "本目录中的 JSON") {
		t.Fatalf("unexpected Chinese examples content: %s", content)
	}
}

func TestTemplateSpecModelsCmdListsAvailableModels(t *testing.T) {
	var requestedPath string
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"models": [
				{
					"modelId": "google/gemini-2.5-flash",
					"displayName": "Gemini 2.5 Flash",
					"provider": "vertex",
					"executionAdapter": "vertex",
					"supportedStepTypes": ["text-generate"],
					"available": true,
					"isDefault": true
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
	cmd := newTemplateSpecModelsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"text-generate"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("models command error = %v", err)
	}
	if requestedPath != "/loom/v1/models" {
		t.Fatalf("path=%q want /loom/v1/models", requestedPath)
	}
	for _, want := range []string{"stepType=text-generate", "onlyAvailable=true"} {
		if !strings.Contains(requestedQuery, want) {
			t.Fatalf("query %q missing %q", requestedQuery, want)
		}
	}
	if strings.Contains(requestedQuery, "provider=") {
		t.Fatalf("query %q should not include provider by default", requestedQuery)
	}
	if !strings.Contains(out.String(), "google/gemini-2.5-flash") {
		t.Fatalf("output missing model id: %s", out.String())
	}
}

func TestTemplateSpecModelsCmdCanFilterProvider(t *testing.T) {
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newTemplateSpecModelsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"text-generate", "--provider", "vertex"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("models command error = %v", err)
	}
	if !strings.Contains(requestedQuery, "provider=vertex") {
		t.Fatalf("query %q missing provider=vertex", requestedQuery)
	}
}

func TestTemplateSpecCreateVersionPostsCanonicalSpec(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "Meta": {"Name": "Spec Test", "Description": "desc"},
  "Steps": [{"StepID": "stp_text01", "DisplayName": "Text", "ExecutionUnit": "text-generate"}],
  "InputSchema": {"Fields": [{"Key": "prompt", "Label": "Prompt", "ValueType": "string"}]},
  "FieldBindings": [{"FieldKey": "prompt", "StepID": "stp_text01", "ParamKey": "prompt", "BindMode": "shared"}]
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	var requestedPath string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"versionId":"ver_123","versionNumber":2,"definitionHash":"hash_123","createdAt":"1777699967"}`))
	}))
	defer server.Close()

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newTemplateSpecCreateVersionCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"tmpl_123", path, "--version-note", "fix judge template"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("create-version command error = %v", err)
	}
	if requestedPath != "/loom/v1/users/me/templates/tmpl_123/versions" {
		t.Fatalf("path=%q want /loom/v1/users/me/templates/tmpl_123/versions", requestedPath)
	}
	if payload["versionNote"] != "fix judge template" {
		t.Fatalf("versionNote=%q", payload["versionNote"])
	}
	spec, ok := payload["canonicalSpec"].(map[string]any)
	if !ok {
		t.Fatalf("canonicalSpec missing or wrong type: %#v", payload["canonicalSpec"])
	}
	if _, ok := spec["meta"]; !ok {
		t.Fatalf("canonicalSpec missing lowerCamel meta: %#v", spec)
	}
	if _, ok := spec["Meta"]; ok {
		t.Fatalf("canonicalSpec should not contain PascalCase Meta: %#v", spec)
	}
	if !strings.Contains(out.String(), `"templateId": "tmpl_123"`) {
		t.Fatalf("output missing template id: %s", out.String())
	}
	if !strings.Contains(out.String(), `"versionNumber": 2`) {
		t.Fatalf("output missing version number: %s", out.String())
	}
}

func TestTemplateSpecCreateCommandsRejectExpandedBeforeRemoteMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expanded.json")
	content := `{
  "meta":{"name":"Legacy expansion"},
  "steps":[{"stepId":"stp_text01","executionUnit":"text-generate"}],
  "inputSchema":{"fields":[{"key":"prompts","label":"Prompts","valueType":"string","multiValue":true,"maxValues":10}]},
  "fieldBindings":[{"fieldKey":"prompts","stepId":"stp_text01","paramKey":"prompt","bindMode":"expanded"}]
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount++
	}))
	defer server.Close()
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}

	tests := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{name: "create", cmd: newTemplateSpecCreateCmd(opts), args: []string{path}},
		{name: "create-version", cmd: newTemplateSpecCreateVersionCmd(opts), args: []string{"tmpl_123", path}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cmd.SetArgs(tt.args)
			err := tt.cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), templateSpecExpandedAuthoringPolicyCode) {
				t.Fatalf("command error = %v, want %s", err, templateSpecExpandedAuthoringPolicyCode)
			}
		})
	}
	if requestCount != 0 {
		t.Fatalf("remote request count = %d, want 0", requestCount)
	}
}

func TestTemplateSpecModelsCmdCanIncludeUnavailableModels(t *testing.T) {
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newTemplateSpecModelsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"image-generate", "--include-unavailable"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("models command error = %v", err)
	}
	if !strings.Contains(requestedQuery, "onlyAvailable=false") {
		t.Fatalf("query %q missing onlyAvailable=false", requestedQuery)
	}
	if !strings.Contains(out.String(), `"models": []`) {
		t.Fatalf("json output missing models array: %s", out.String())
	}
}

func TestTemplateSpecSubmitWorkbookSendsFilename(t *testing.T) {
	workbookPath := filepath.Join(t.TempDir(), "custom-input.xlsx")
	if err := os.WriteFile(workbookPath, []byte("xlsx bytes"), 0o644); err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	var submitFilename string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch r.URL.Path {
		case "/loom/v1/users/me/templates/tmpl_123/versions/ver_123:validateWorkbook":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/loom/v1/users/me/templates/tmpl_123/versions/ver_123:runWorkbook":
			submitFilename, _ = payload["filename"].(string)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"runId":"run_123","status":"pending","acceptedAt":"1777699967"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newTemplateSpecSubmitWorkbookCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"tmpl_123", "ver_123", workbookPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("submit-workbook command error = %v", err)
	}
	if submitFilename != "custom-input.xlsx" {
		t.Fatalf("filename=%q want custom-input.xlsx", submitFilename)
	}
	if !strings.Contains(out.String(), `"runId": "run_123"`) {
		t.Fatalf("output missing run id: %s", out.String())
	}
}

func TestTemplateSpecSubmitWorkbookPrintsGeneratedClientRequestID(t *testing.T) {
	workbookPath := filepath.Join(t.TempDir(), "custom-input.xlsx")
	if err := os.WriteFile(workbookPath, []byte("xlsx bytes"), 0o644); err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/users/me/templates/tmpl_123/versions/ver_123:validateWorkbook":
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/loom/v1/users/me/templates/tmpl_123/versions/ver_123:runWorkbook":
			http.Error(w, `{"error":"temporary failure"}`, http.StatusServiceUnavailable)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newTemplateSpecSubmitWorkbookCmd(opts)
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"tmpl_123", "ver_123", workbookPath})

	if err := cmd.Execute(); err == nil {
		t.Fatal("submit-workbook error = nil, want request failure")
	}
	if !strings.Contains(stderr.String(), "clientRequestId: loomloom-cli-") {
		t.Fatalf("stderr=%q want generated clientRequestId before request failure", stderr.String())
	}
}

func TestTemplateSpecPrecheckUsesProductAPI(t *testing.T) {
	var request map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/users/me/templates/tmpl_123:precheck" {
			t.Fatalf("path=%q want private template precheck endpoint", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"estimatedTotalCostT":119350,
			"balanceCheck":{
				"currency":"CNY",
				"availableBalance":999262000,
				"isSufficient":true
			}
		}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newTemplateSpecPrecheckCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"tmpl_123",
		"--version-id", "ver_123",
		"--input-file-id", "ec1015c0-5078-4409-84b5-b46ddc3e9312",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("template-spec precheck command error = %v", err)
	}
	if request["versionId"] != "ver_123" {
		t.Fatalf("versionId=%v want ver_123", request["versionId"])
	}
	if request["inputFileId"] != "ec1015c0-5078-4409-84b5-b46ddc3e9312" {
		t.Fatalf("inputFileId=%v want uploaded input file id", request["inputFileId"])
	}
	for _, want := range []string{
		"estimated_cost",
		"CNY 0.0119",
		"available_balance",
		"CNY 99.9262",
		"sufficient",
		"true",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%q want %q", out.String(), want)
		}
	}
}

func TestTemplateSpecPrecheckJSONKeepsEstimatedTotalCostT(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/users/me/templates/tmpl_123:precheck" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"estimatedTotalCostT":119350,"balanceCheck":{"currency":"CNY","availableBalance":999262000,"isSufficient":true}}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newTemplateSpecPrecheckCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"tmpl_123",
		"--version-id", "ver_123",
		"--input-file-id", "input-file-1",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("template-spec precheck command error = %v", err)
	}
	if !strings.Contains(out.String(), `"estimatedTotalCostT": 119350`) {
		t.Fatalf("output=%s want estimatedTotalCostT", out.String())
	}
	if strings.Contains(out.String(), `"estimatedTotalCost":`) {
		t.Fatalf("output=%s must not emit estimatedTotalCost", out.String())
	}
}

func TestTemplateSpecPrecheckInsufficientBalanceUsesBoundPlatformMessage(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{Platform: platform.ShengSuanYun}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/users/me/templates/tmpl_123:precheck" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"estimatedTotalCostT":119350,"balanceCheck":{"currency":"CNY","availableBalance":0,"isSufficient":false}}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newTemplateSpecPrecheckCmd(opts)
	cmd.SetArgs([]string{
		"tmpl_123",
		"--version-id", "ver_123",
		"--input-file-id", "input-file-1",
	})

	err := cmd.Execute()
	if err == nil || err.Error() != insufficientShengSuanYunBalanceMessage {
		t.Fatalf("error=%v want fixed ShengSuanYun balance message", err)
	}
}

func TestTemplateSpecPrecheckRejectsInputAssetID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called with an inputAssetId")
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newTemplateSpecPrecheckCmd(opts)
	cmd.SetArgs([]string{
		"tmpl_123",
		"--version-id", "ver_123",
		"--input-file-id", "ia_example",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("template-spec precheck error = nil, want inputAssetId rejection")
	}
	if !strings.Contains(err.Error(), "orchestrationInputs:upload") ||
		!strings.Contains(err.Error(), "inputAssets:upload") {
		t.Fatalf("error=%q want upload endpoint guidance", err)
	}
}

func TestTemplateSpecPrecheckWorkbookUsesProductAPI(t *testing.T) {
	workbookPath := filepath.Join(t.TempDir(), "custom-input.xlsx")
	if err := os.WriteFile(workbookPath, []byte("xlsx bytes"), 0o644); err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	var request struct {
		Filename string `json:"filename"`
		Content  []byte `json:"content"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/users/me/templates/tmpl_123/versions/ver_123:precheckWorkbook" {
			t.Fatalf("path=%q want private template workbook precheck endpoint", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"estimatedTotalCostT":119350,"balanceCheck":{"currency":"CNY","availableBalance":999262000,"isSufficient":true}}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newTemplateSpecPrecheckWorkbookCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"tmpl_123", "ver_123", workbookPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("template-spec precheck-workbook command error = %v", err)
	}
	if request.Filename != "custom-input.xlsx" {
		t.Fatalf("filename=%q want custom-input.xlsx", request.Filename)
	}
	if !bytes.Equal(request.Content, []byte("xlsx bytes")) {
		t.Fatalf("content=%q want workbook bytes", string(request.Content))
	}
	if !strings.Contains(out.String(), "estimated_cost") || !strings.Contains(out.String(), "CNY 0.0119") {
		t.Fatalf("output=%q want formatted estimated cost", out.String())
	}
}

func TestTemplateSpecRunPrintsGeneratedClientRequestIDBeforeRequestFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/users/me/templates/tmpl_123:run" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		http.Error(w, `{"error":"temporary failure"}`, http.StatusServiceUnavailable)
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newTemplateSpecRunCmd(opts)
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"tmpl_123",
		"--version-id", "ver_123",
		"--input-file-id", "ec1015c0-5078-4409-84b5-b46ddc3e9312",
	})

	if err := cmd.Execute(); err == nil {
		t.Fatal("template-spec run error = nil, want request failure")
	}
	if !strings.Contains(stderr.String(), "clientRequestId: loomloom-cli-") {
		t.Fatalf("stderr=%q want generated clientRequestId before request failure", stderr.String())
	}
}

func TestTemplateSpecRunRejectsInputAssetID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called with an inputAssetId")
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newTemplateSpecRunCmd(opts)
	cmd.SetArgs([]string{
		"tmpl_123",
		"--version-id", "ver_123",
		"--input-file-id", "ia_example",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("template-spec run error = nil, want inputAssetId rejection")
	}
	if !strings.Contains(err.Error(), "orchestrationInputs:upload") ||
		!strings.Contains(err.Error(), "inputAssets:upload") {
		t.Fatalf("error=%q want upload endpoint guidance", err)
	}
}

func TestTemplateSpecCheckRejectsInvalidStepID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid-step-id.json")
	content := `{
		"meta":{"name":"Invalid"},
		"steps":[{"stepId":"stp_text","executionUnit":"text-generate"}],
		"inputSchema":{"fields":[{"key":"prompt","label":"Prompt","valueType":"string"}]}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	opts := &rootOptions{output: "json"}
	cmd := newTemplateSpecCheckCmd(opts)
	cmd.SetArgs([]string{path})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "must match stp_<6-10 base36 chars>") {
		t.Fatalf("error=%v want invalid step ID", err)
	}
}

func TestTemplateSpecCheckRejectsUnwrappedSampleRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid-sample-rows.json")
	content := `{
		"meta":{"name":"Invalid"},
		"steps":[{"stepId":"stp_text01","executionUnit":"text-generate"}],
		"inputSchema":{
			"fields":[{"key":"prompt","label":"Prompt","valueType":"string"}],
			"sampleRows":[{"prompt":"hello"}]
		}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	opts := &rootOptions{output: "json"}
	cmd := newTemplateSpecCheckCmd(opts)
	cmd.SetArgs([]string{path})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "must wrap field values in a values object") {
		t.Fatalf("error=%v want invalid sample row shape", err)
	}
}
