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

func TestTemplateSpecCheckCmdUsesCurrentServerAuthority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	// This is valid JSON but intentionally fails the embedded semantic validator.
	// The check command must still send it to the server, which owns authority.
	content := `{"meta":{},"steps":[]}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	var requestedPath string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode validation request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"valid":true,"primaryOutputType":"markdown","definitionHash":"hash-def","contractBundleHash":"hash-bundle","authorityFingerprint":"hash-bundle"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "text"}
	cmd := newTemplateSpecCheckCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("check command error = %v", err)
	}
	if requestedPath != "/loom/v1/templateSpecs:validate" {
		t.Fatalf("path=%q", requestedPath)
	}
	if payload["specVersion"] != "template-spec/v2" {
		t.Fatalf("specVersion=%#v", payload["specVersion"])
	}
	if !strings.Contains(out.String(), "authority_fingerprint\thash-bundle") {
		t.Fatalf("output missing server authority result: %s", out.String())
	}
}

func TestTemplateSpecCheckCmdReturnsServerRejection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, []byte(`{"meta":{"name":"T"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"subject revision is not enabled"}`, http.StatusBadRequest)
	}))
	defer server.Close()
	cmd := newTemplateSpecCheckCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "subject revision is not enabled") {
		t.Fatalf("error=%v", err)
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
	for _, want := range []string{"# TemplateSpec v2 syntax", "lowerCamel", "templateInputs", "fixedModelContract", "inputBindings", "stepOutput", "authoring-context"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
}

func TestTemplateSpecAuthoringContextCmdUsesCurrentServerContext(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"profiles":[{"profileId":"text.vision.openai-chat.v1","revision":"2026-08-25.1","canonicalHash":"sha256:profile","capability":"text","endpoint":"/v1/chat/completions","compiler":"gateway-openai-chat-vision-v1","stream":false,"inputPorts":[{"portId":"prompt","kind":"value","valueType":"string","required":true},{"portId":"image","kind":"artifact","required":true,"acceptedMimeTypes":["image/jpeg","image/png","image/webp"],"minItems":1,"maxItems":1}],"output":{"text":true,"usage":true},"eligibleModels":[{"modelId":"google/gemini-3-flash"}]}]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newTemplateSpecAuthoringContextCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("authoring-context command error = %v", err)
	}
	if requestedPath != "/loom/v1/templateAuthoringContext" {
		t.Fatalf("path=%q want /loom/v1/templateAuthoringContext", requestedPath)
	}
	for _, want := range []string{"text.vision.openai-chat.v1", "2026-08-25.1", "google/gemini-3-flash", `"kind": "artifact"`, `"acceptedMimeTypes": [`, `"image/png"`, `"minItems": 1`, `"maxItems": 1`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q: %s", want, out.String())
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
					"authoringOptions": [{"kind":"capabilityProfile","capabilityProfile":{"profileId":"text.basic.openai-chat.v1","profileRevision":"2026-08-15.1"}}],
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
	for _, want := range []string{"stepType=text-generate"} {
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
		_, _ = w.Write([]byte(`{"models":[{"modelId":"vertex/gemini","authoringOptions":[{"kind":"capabilityProfile"}]},{"modelId":"other/model","authoringOptions":[{"kind":"fixedModelContract"}]}]}`))
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
	if strings.Contains(requestedQuery, "provider=") {
		t.Fatalf("query %q must not send the unsupported provider parameter", requestedQuery)
	}
	if !strings.Contains(out.String(), "vertex/gemini") || strings.Contains(out.String(), "other/model") {
		t.Fatalf("provider filter output is incorrect: %s", out.String())
	}
}

func TestTemplateSpecContractsCmdListsAuthoringContract(t *testing.T) {
	var requestedPath string
	var requestedModelID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedModelID = r.URL.Query().Get("modelId")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"contracts":[{"subjectRevisionId":"subject-image-v2","subjectHash":"sha256:abc","modelId":"ali/qwen-image-plus","operation":"text-to-image","variant":"base","executionUnitRef":"image-generate","inputPorts":[{"portId":"prompt","kind":"value","valueType":"string","required":true}]}]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "text"}
	cmd := newTemplateSpecContractsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"ali/qwen-image-plus"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("contracts command error = %v", err)
	}
	if requestedPath != "/loom/v1/modelContracts" {
		t.Fatalf("path=%q want /loom/v1/modelContracts", requestedPath)
	}
	if requestedModelID != "ali/qwen-image-plus" {
		t.Fatalf("modelId=%q", requestedModelID)
	}
	for _, want := range []string{"text-to-image", "subject-image-v2", "image-generate", "prompt"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q: %s", want, out.String())
		}
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

	var requestedPaths []string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		var current map[string]any
		if err := json.NewDecoder(r.Body).Decode(&current); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/loom/v1/templateSpecs:validate" {
			_, _ = w.Write([]byte(`{"valid":true,"primaryOutputType":"markdown","definitionHash":"hash_123","contractBundleHash":"hash_bundle","authorityFingerprint":"hash_bundle"}`))
			return
		}
		payload = current
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
	if len(requestedPaths) != 2 || requestedPaths[0] != "/loom/v1/templateSpecs:validate" || requestedPaths[1] != "/loom/v1/users/me/templates/tmpl_123/versions" {
		t.Fatalf("paths=%v", requestedPaths)
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

func TestTemplateSpecGetVersionExportsHistoricalCanonicalSpec(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"templateId":"tmpl_123","versionId":"ver_002","versionNumber":2,"specVersion":"template-spec/v2","canonicalSpec":{"meta":{"name":"Historical"},"steps":[]},"definitionHash":"hash-def","createdAtUnix":1710000000}`))
	}))
	defer server.Close()
	target := filepath.Join(t.TempDir(), "historical.json")
	cmd := newTemplateSpecGetVersionCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"tmpl_123", "ver_002", "--output-file", target})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("get-version command error = %v", err)
	}
	if requestedPath != "/loom/v1/users/me/templates/tmpl_123/versions/ver_002" {
		t.Fatalf("path=%q", requestedPath)
	}
	written, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read exported spec: %v", err)
	}
	if !strings.Contains(string(written), `"name": "Historical"`) {
		t.Fatalf("exported spec=%s", written)
	}
	if !strings.Contains(out.String(), `"definitionHash": "hash-def"`) || !strings.Contains(out.String(), `"path":`) {
		t.Fatalf("output=%s", out.String())
	}
}

func TestTemplateSpecGetVersionPrintsCanonicalSpecWithoutOutputFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"templateId":"tmpl_123","versionId":"ver_001","versionNumber":1,"specVersion":"template-spec/v1","canonicalSpec":{"Meta":{"Name":"Legacy"}},"definitionHash":"hash-v1"}`))
	}))
	defer server.Close()
	cmd := newTemplateSpecGetVersionCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"tmpl_123", "ver_001"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("get-version command error = %v", err)
	}
	if !strings.Contains(out.String(), `"canonicalSpec":`) || !strings.Contains(out.String(), `"Name": "Legacy"`) {
		t.Fatalf("output=%s", out.String())
	}
}

func TestTemplateSpecCreateCommandsRejectV1BeforeRemoteMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.json")
	content := `{"meta":{"name":"Legacy"},"steps":[{"stepId":"stp_text01","executionUnit":"text-generate"}],"inputSchema":{"fields":[]}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		http.Error(w, `{"error":"TemplateSpec v2 schema validation failed: legacy v1 is read-only"}`, http.StatusBadRequest)
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
	if len(requestedPaths) != 2 {
		t.Fatalf("validation request count = %d, want 2", len(requestedPaths))
	}
	for _, requestedPath := range requestedPaths {
		if requestedPath != "/loom/v1/templateSpecs:validate" {
			t.Fatalf("unexpected mutation path: %s", requestedPath)
		}
	}
}

func TestTemplateSpecModelsCmdRejectsIncludeUnavailableModels(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requestCount++ }))
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

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "no longer supported") {
		t.Fatalf("models command error = %v, want unsupported semantic", err)
	}
	if requestCount != 0 {
		t.Fatalf("request count=%d, want 0", requestCount)
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
				"availability":"settled_only",
				"finalAdmission":"gateway",
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
		_, _ = w.Write([]byte(`{"estimatedTotalCostT":119350,"balanceCheck":{"currency":"CNY","availableBalance":999262000,"availability":"settled_only","finalAdmission":"gateway","isSufficient":true}}`))
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"TemplateSpec v2 schema validation failed: invalid step ID"}`, http.StatusBadRequest)
	}))
	defer server.Close()
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"TemplateSpec v2 schema validation failed: invalid sample row shape"}`, http.StatusBadRequest)
	}))
	defer server.Close()
	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newTemplateSpecCheckCmd(opts)
	cmd.SetArgs([]string{path})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "TemplateSpec v2 schema validation failed") {
		t.Fatalf("error=%v want invalid sample row shape", err)
	}
}
