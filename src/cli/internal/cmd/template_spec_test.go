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

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/platform"
	templatespecdocs "github.com/cogfoundry-labs/loomloom/src/cli/internal/template_spec_docs"
	"github.com/spf13/cobra"
)

func TestGeneratedTemplateSpecExamplesAreEnglish(t *testing.T) {
	root := findRepoRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, "docs", "ir-spec", "en", "examples", "*", "*.json"))
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
	paths, err := filepath.Glob(filepath.Join(root, "docs", "ir-spec", "en", "examples", "valid", "*.json"))
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

func TestGeneratedLegacyV1ExampleFailsNewAuthoringValidation(t *testing.T) {
	path := filepath.Join(findRepoRoot(t), "docs", "ir-spec", "en", "examples", "invalid", "legacy-v1-shape.json")
	if _, _, err := loadTemplateSpecFile(path); err == nil || !strings.Contains(err.Error(), "TemplateSpec v2 schema validation failed") {
		t.Fatalf("loadTemplateSpecFile() error = %v, want v2 schema rejection", err)
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
  "meta": {"name": "Spec Test", "description": "desc"},
  "templateInputs": {
    "prompt": {"kind":"value","valueType":"string","required":true,"blankPolicy":"error","presentation":{"label":"Prompt","order":10}}
  },
  "steps": [{
    "stepId":"stp_text01","displayName":"Text",
    "executionBinding":{"kind":"fixedModelContract","subjectRevisionId":"subject-text-v2"},
    "inputBindings":{"prompt":{"source":"templateInput","inputKey":"prompt"}}
  }],
  "workbook": {}
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
	if len(spec.TemplateInputs) != 1 || len(spec.Steps[0].InputBindings) != 1 {
		t.Fatalf("unexpected v2 input/binding counts: %#v", spec)
	}
	if !strings.Contains(string(raw), `"templateInputs"`) {
		t.Fatalf("expected exact lowerCamel TemplateSpec v2 JSON, got %s", string(raw))
	}
}

func TestLoadTemplateSpecFile_RejectsV1ForNewAuthoring(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{"meta":{"name":"Legacy"},"steps":[{"stepId":"stp_text01","executionUnit":"text-generate"}],"inputSchema":{"fields":[]}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if _, _, err := loadTemplateSpecFile(path); err == nil || !strings.Contains(err.Error(), "TemplateSpec v2 schema validation failed") {
		t.Fatalf("loadTemplateSpecFile() error = %v, want v2 schema rejection", err)
	}
}

func TestLoadTemplateSpecFile_AllowsArtifactCollection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "meta":{"name":"Reference images"},
  "templateInputs":{"references":{"kind":"artifact","required":false,"blankPolicy":"omit","acceptedMimeTypes":["image/*"],"minItems":0,"maxItems":10,"presentation":{"label":"References","order":10}}},
	"steps":[{
    "stepId":"stp_image01","displayName":"Edit image",
    "executionBinding":{"kind":"fixedModelContract","subjectRevisionId":"subject-image-v2"},
    "inputBindings":{"images":{"source":"templateInput","inputKey":"references"}}
	}],
	"workbook":{}
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
	content := `{"meta":{},"steps":[{"stepId":"stp_text01","displayName":"Text","executionBinding":{"kind":"fixedModelContract","subjectRevisionId":"subject-text-v2"}}],"workbook":{}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	if _, _, err := loadTemplateSpecFile(path); err == nil {
		t.Fatal("loadTemplateSpecFile() error = nil, want missing name error")
	}
}

func TestLoadTemplateSpecFile_RejectsPascalCaseInsteadOfNormalizing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{"Meta":{"Name":"Pascal case"},"Steps":[],"Workbook":{}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	if _, _, err := loadTemplateSpecFile(path); err == nil || !strings.Contains(err.Error(), "TemplateSpec v2 schema validation failed") {
		t.Fatalf("loadTemplateSpecFile() error = %v, want exact-wire rejection", err)
	}
}

func TestLoadTemplateSpecFile_AllowsComposeValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "meta":{"name":"Compose"},
  "templateInputs":{"product":{"kind":"value","valueType":"string","required":true,"blankPolicy":"error","presentation":{"label":"Product","order":10}}},
  "steps":[{"stepId":"stp_text01","displayName":"Text","executionBinding":{"kind":"fixedModelContract","subjectRevisionId":"subject-text-v2"},"inputBindings":{"prompt":{"source":"composeValue","compose":{"kind":"concat","parts":[{"source":"literal","literal":"Describe"},{"source":"templateInput","inputKey":"product"}]}}}}],
  "workbook":{}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	spec, _, err := loadTemplateSpecFile(path)
	if err != nil {
		t.Fatalf("loadTemplateSpecFile() error = %v", err)
	}
	if spec.Steps[0].InputBindings["prompt"].Source != "composeValue" {
		t.Fatalf("unexpected prompt binding: %#v", spec.Steps[0].InputBindings["prompt"])
	}
}

func TestLoadTemplateSpecFile_RejectsStepOutputWithoutDependency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "meta":{"name":"Missing dependency"},
  "steps":[
    {"stepId":"stp_source1","displayName":"Source","executionBinding":{"kind":"fixedModelContract","subjectRevisionId":"subject-a"}},
    {"stepId":"stp_target1","displayName":"Target","executionBinding":{"kind":"fixedModelContract","subjectRevisionId":"subject-b"},"inputBindings":{"image":{"source":"stepOutput","stepId":"stp_source1","portId":"output"}}}
  ],
  "workbook":{}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	_, _, err := loadTemplateSpecFile(path)
	if err == nil || !strings.Contains(err.Error(), "must appear in dependsOn") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadTemplateSpecFile_AllowsThreeVisibleFieldParamBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{"meta":{"name":"Three inputs"},"templateInputs":{"body":{"kind":"value","valueType":"string","required":true,"blankPolicy":"error","presentation":{"label":"Body","order":10}},"style":{"kind":"value","valueType":"string","required":true,"blankPolicy":"error","presentation":{"label":"Style","order":20}},"format":{"kind":"value","valueType":"string","required":true,"blankPolicy":"error","presentation":{"label":"Format","order":30}}},"steps":[{"stepId":"stp_text01","displayName":"Text","executionBinding":{"kind":"fixedModelContract","subjectRevisionId":"subject-text-v2"},"inputBindings":{"prompt":{"source":"composeValue","compose":{"kind":"concat","separator":"\n\n","parts":[{"source":"templateInput","inputKey":"body"},{"source":"templateInput","inputKey":"style"},{"source":"templateInput","inputKey":"format"}]}}}}],"workbook":{}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	spec, raw, err := loadTemplateSpecFile(path)
	if err != nil {
		t.Fatalf("loadTemplateSpecFile() error = %v", err)
	}
	if len(spec.TemplateInputs) != 3 || !strings.Contains(string(raw), `"inputKey":"format"`) {
		t.Fatalf("v2 compose inputs were not preserved: %s", string(raw))
	}
}

func TestTemplateSpecCheckCmdCountsInputBindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{"meta":{"name":"Binding count"},"templateInputs":{"prompt":{"kind":"value","valueType":"string","required":true,"blankPolicy":"error","presentation":{"label":"Prompt","order":10}}},"steps":[{"stepId":"stp_text01","displayName":"Text","executionBinding":{"kind":"fixedModelContract","subjectRevisionId":"subject-text-v2"},"inputBindings":{"prompt":{"source":"templateInput","inputKey":"prompt"}}}],"workbook":{}}`
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
	for _, want := range []string{"# Understand TemplateSpec v2", "<stepId>.<portId>", "frozen version"} {
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
	for _, want := range []string{"# Quickstart", "template-spec/v2", "canonicalSpecV2", "subjectRevisionId", "template-spec check"} {
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
	for _, want := range []string{"# TemplateSpec v2 syntax", "lowerCamel", "templateInputs", "fixedModelContract", "Input binding sources", "stepOutput"} {
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
	for _, want := range []string{"Language: zh-CN", "# TemplateSpec v2 语法参考", "顶层对象", "inputBindings"} {
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
	for _, want := range []string{"# TemplateSpec v2 examples", "multi-step-fixed-model", "artifact-merge", "compose-value", "content-sequence", "capability-profile"} {
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
	if content, _ := payload["content"].(string); !strings.Contains(content, "# TemplateSpec v2 示例") || !strings.Contains(content, "canonicalSpecV2") {
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

func TestTemplateSpecCreateVersionPostsCanonicalSpecV2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	content := `{
  "meta":{"name":"Spec Test","description":"desc"},
  "templateInputs":{"prompt":{"kind":"value","valueType":"string","required":true,"blankPolicy":"error","presentation":{"label":"Prompt","order":10}}},
  "steps":[{"stepId":"stp_text01","displayName":"Text","executionBinding":{"kind":"fixedModelContract","subjectRevisionId":"subject-text-v2"},"inputBindings":{"prompt":{"source":"templateInput","inputKey":"prompt"}}}],
  "workbook":{}
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
	if payload["specVersion"] != "template-spec/v2" {
		t.Fatalf("specVersion=%#v, want template-spec/v2", payload["specVersion"])
	}
	spec, ok := payload["canonicalSpecV2"].(map[string]any)
	if !ok {
		t.Fatalf("canonicalSpecV2 missing or wrong type: %#v", payload["canonicalSpecV2"])
	}
	if _, ok := spec["meta"]; !ok {
		t.Fatalf("canonicalSpecV2 missing lowerCamel meta: %#v", spec)
	}
	if _, ok := spec["Meta"]; ok {
		t.Fatalf("canonicalSpecV2 should not contain PascalCase Meta: %#v", spec)
	}
	if _, exists := payload["canonicalSpec"]; exists {
		t.Fatalf("retired canonicalSpec must not be sent: %#v", payload)
	}
	if !strings.Contains(out.String(), `"templateId": "tmpl_123"`) {
		t.Fatalf("output missing template id: %s", out.String())
	}
	if !strings.Contains(out.String(), `"versionNumber": 2`) {
		t.Fatalf("output missing version number: %s", out.String())
	}
}

func TestTemplateSpecCreateCommandsRejectV1BeforeRemoteMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.json")
	content := `{"meta":{"name":"Legacy"},"steps":[{"stepId":"stp_text01","executionUnit":"text-generate"}],"inputSchema":{"fields":[]}}`
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
			if err == nil || !strings.Contains(err.Error(), "TemplateSpec v2 schema validation failed") {
				t.Fatalf("command error = %v, want v2 schema rejection", err)
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
		"steps":[{"stepId":"stp_text","displayName":"Text","executionBinding":{"kind":"fixedModelContract","subjectRevisionId":"subject-text-v2"}}],
		"workbook":{}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	opts := &rootOptions{output: "json"}
	cmd := newTemplateSpecCheckCmd(opts)
	cmd.SetArgs([]string{path})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "TemplateSpec v2 schema validation failed") {
		t.Fatalf("error=%v want invalid step ID", err)
	}
}

func TestTemplateSpecCheckRejectsUnwrappedSampleRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid-sample-rows.json")
	content := `{
		"meta":{"name":"Invalid"},
		"templateInputs":{"prompt":{"kind":"value","valueType":"string","required":true,"blankPolicy":"error","presentation":{"label":"Prompt","order":10}}},
		"steps":[{"stepId":"stp_text01","displayName":"Text","executionBinding":{"kind":"fixedModelContract","subjectRevisionId":"subject-text-v2"}}],
		"workbook":{"sampleRows":[{"prompt":"hello"}]}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	opts := &rootOptions{output: "json"}
	cmd := newTemplateSpecCheckCmd(opts)
	cmd.SetArgs([]string{path})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "TemplateSpec v2 schema validation failed") {
		t.Fatalf("error=%v want invalid sample row shape", err)
	}
}
