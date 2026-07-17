package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var bundledSkillDirs = []string{
	"skills/codex/loomloom",
	"skills/claude/loomloom",
	"skills/openclaw/loomloom",
}

const canonicalSkillReferencesDir = "skill-sources/references"

func TestTemplateSpecUploadedTextFixtureContract(t *testing.T) {
	root := findRepoRoot(t)
	caseData, err := os.ReadFile(filepath.Join(root, "cli/internal/cmd/testdata/template-spec-authoring/uploaded-text.json"))
	if err != nil {
		t.Fatalf("read authoring case: %v", err)
	}
	var authoringCase struct {
		Request  string `json:"request"`
		Fixture  string `json:"fixture"`
		Expected struct {
			FieldKey         string `json:"fieldKey"`
			ValueType        string `json:"valueType"`
			AcceptedMimeType string `json:"acceptedMimeType"`
			InputPort        string `json:"inputPort"`
			SourceType       string `json:"sourceType"`
		} `json:"expected"`
	}
	if err := json.Unmarshal(caseData, &authoringCase); err != nil {
		t.Fatalf("decode authoring case: %v", err)
	}
	if authoringCase.Request == "" {
		t.Fatal("authoring case request must describe the user intent")
	}

	specData, err := os.ReadFile(filepath.Join(root, authoringCase.Fixture))
	if err != nil {
		t.Fatalf("read fixture spec: %v", err)
	}
	var spec struct {
		Steps []struct {
			UpstreamBindings []struct {
				InputPort      string `json:"inputPort"`
				SourceType     string `json:"sourceType"`
				SourceInputKey string `json:"sourceInputKey"`
			} `json:"upstreamBindings"`
		} `json:"steps"`
		InputSchema struct {
			Fields []struct {
				Key               string   `json:"key"`
				ValueType         string   `json:"valueType"`
				AcceptedMimeTypes []string `json:"acceptedMimeTypes"`
			} `json:"fields"`
		} `json:"inputSchema"`
		FieldBindings []struct {
			FieldKey string `json:"fieldKey"`
		} `json:"fieldBindings"`
	}
	if err := json.Unmarshal(specData, &spec); err != nil {
		t.Fatalf("decode fixture spec: %v", err)
	}
	if len(spec.InputSchema.Fields) != 1 {
		t.Fatalf("fixture fields = %d, want 1", len(spec.InputSchema.Fields))
	}
	field := spec.InputSchema.Fields[0]
	if field.Key != authoringCase.Expected.FieldKey || field.ValueType != authoringCase.Expected.ValueType {
		t.Fatalf("fixture field = (%q, %q), want (%q, %q)", field.Key, field.ValueType, authoringCase.Expected.FieldKey, authoringCase.Expected.ValueType)
	}
	if len(field.AcceptedMimeTypes) != 1 || field.AcceptedMimeTypes[0] != authoringCase.Expected.AcceptedMimeType {
		t.Fatalf("acceptedMimeTypes = %v, want [%q]", field.AcceptedMimeTypes, authoringCase.Expected.AcceptedMimeType)
	}
	if len(spec.Steps) != 1 || len(spec.Steps[0].UpstreamBindings) != 1 {
		t.Fatalf("fixture upstream bindings = %#v, want exactly one", spec.Steps)
	}
	binding := spec.Steps[0].UpstreamBindings[0]
	if binding.InputPort != authoringCase.Expected.InputPort || binding.SourceType != authoringCase.Expected.SourceType || binding.SourceInputKey != field.Key {
		t.Fatalf("fixture upstream binding = %#v, want inputPort=%q sourceType=%q sourceInputKey=%q", binding, authoringCase.Expected.InputPort, authoringCase.Expected.SourceType, field.Key)
	}
	for _, fieldBinding := range spec.FieldBindings {
		if fieldBinding.FieldKey == field.Key {
			t.Fatalf("text_reference field %q must not be composed into a prompt field binding", field.Key)
		}
	}
}

func TestBundledSkillsUseDoctorPlatformFacts(t *testing.T) {
	root := findRepoRoot(t)
	text := readCanonicalSkillReference(t, root, "setup.md")
	for _, want := range []string{
		"loomloom doctor --output json",
		"credential_action",
		"你还没有完整配置 LoomLoom Server 和密钥。请选择要使用的平台：",
		"https://loomloom.shengsuanyun.com/loom/v1",
		"https://console.shengsuanyun.com/user/keys",
		"当前未检测到胜算云密钥。请前往胜算云控制台创建或复制密钥后配置到本地环境：",
		"当前胜算云账户余额不足，请前往胜算云控制台充值后再继续：",
		"https://console.shengsuanyun.com/user/recharge",
		"在 CogFoundry 计费功能上线前，请使用胜算云控制台创建 API 密钥",
		"CogFoundry 面向新加坡及其他海外地区用户，当前支付和交易能力仍在建设中，敬请期待。当前阶段请继续使用胜算云。",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("%s missing %q", canonicalSkillReferencesDir, want)
		}
	}
	for _, forbidden := range []string{
		"https://cogfoundry.ai",
		"https://console-dev.cogfoundry",
		"https://console.cogfoundry",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("%s should not include CogFoundry console URL %q", canonicalSkillReferencesDir, forbidden)
		}
	}
}

func TestBundledSkillsUseCanonicalUploadedTextRules(t *testing.T) {
	root := findRepoRoot(t)
	text := readCanonicalSkillReference(t, root, "template-spec.md")
	for _, want := range []string{
		"TS-IN-001",
		"TS-IN-002",
		"TS-IN-003",
		"whether future users will paste the content or upload a file",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("%s missing %q", canonicalSkillReferencesDir, want)
		}
	}
}

func TestBundledSkillsExposeTemplateSpecDocsLanguageOption(t *testing.T) {
	root := findRepoRoot(t)
	text := readCanonicalSkillReference(t, root, "template-spec.md")
	for _, want := range []string{
		"TemplateSpec docs default to English and are also available in Chinese",
		"loomloom template-spec docs spec --lang zh-CN",
		"Select the documentation language as appropriate for the conversation and task",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("%s missing %q", canonicalSkillReferencesDir, want)
		}
	}
}

func TestBundledSkillReferenceLayout(t *testing.T) {
	root := findRepoRoot(t)
	expected := []string{
		"billing.md",
		"cli.md",
		"execution.md",
		"local-skills.md",
		"market.md",
		"setup.md",
		"template-spec.md",
	}
	entries, err := os.ReadDir(filepath.Join(root, canonicalSkillReferencesDir))
	if err != nil {
		t.Fatalf("read canonical references directory: %v", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if strings.Join(names, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("canonical reference files=%v, want %v", names, expected)
	}
	for _, name := range expected {
		if strings.TrimSpace(readCanonicalSkillReference(t, root, name)) == "" {
			t.Fatalf("canonical reference %s is empty", name)
		}
	}

	for _, rel := range bundledSkillDirs {
		t.Run(rel, func(t *testing.T) {
			skillData, err := os.ReadFile(filepath.Join(root, rel, "SKILL.md"))
			if err != nil {
				t.Fatalf("read SKILL.md: %v", err)
			}
			skillText := string(skillData)

			for _, name := range expected {
				if !strings.Contains(skillText, "references/"+name) {
					t.Fatalf("SKILL.md does not route to references/%s", name)
				}
			}
		})
	}
}

func readCanonicalSkillReference(t *testing.T, root, reference string) string {
	t.Helper()
	referenceData, err := os.ReadFile(filepath.Join(root, canonicalSkillReferencesDir, reference))
	if err != nil {
		t.Fatalf("read canonical Skill reference %s: %v", reference, err)
	}
	return string(referenceData)
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error=%v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "skills")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root with skills directory not found")
		}
		dir = parent
	}
}
