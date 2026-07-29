package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"
)

var bundledSkillDirs = []string{
	"skills/codex/loomloom",
	"skills/claude/loomloom",
	"skills/openclaw/loomloom",
}

const canonicalSkillReferencesDir = "skill-sources/references"

func TestBundledSkillsMatchUserLanguage(t *testing.T) {
	root := findRepoRoot(t)
	for _, rel := range bundledSkillDirs {
		text, err := os.ReadFile(filepath.Join(root, rel, "SKILL.md"))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, want := range []string{
			"Respond in the language evident from the user's messages",
			"default to Chinese for ShengSuanYun and English for CogFoundry",
			"including predefined templates, confirmations, warnings, and errors",
			"only from the user's explicit selection or a successful `loomloom doctor --output json` result",
			"Never infer it from a hostname, location, language, or other context",
		} {
			if !strings.Contains(string(text), want) {
				t.Errorf("%s missing language rule %q", rel, want)
			}
		}
	}

	setup := readCanonicalSkillReference(t, root, "setup.md")
	if !strings.Contains(setup, "required business content, not verbatim wording") {
		t.Errorf("%s/setup.md must mark fixed messages as localizable business content", canonicalSkillReferencesDir)
	}
}

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
		"If Doctor reports `healthy=true`, continue with the existing credential",
		"API Token authentication is available for every platform",
		"Browser login is available only for ShengSuanYun",
		"user's explicit platform selection or the already selected Server profile reported by Doctor",
		"neither explicitly selected a platform nor provided a Server",
		"present both preset platforms",
		"Do not start either platform's authentication flow before the choice",
		"an unbound `LOOMLOOM_TOKEN` without a verified Server profile",
		"the LoomLoom repository owner or download source",
		"the installed CLI version, release channel, or apparent documentation maturity",
		"the user's language, location, region, or a platform recommendation",
		"Do not answer with only one platform's credential URL",
		"show both preset platforms and ask the user to select one before starting authentication",
		"Do not run `loomloom login` yet",
		"After the user selects ShengSuanYun",
		"loomloom login",
		"After the user selects CogFoundry or a custom platform",
		"After either authentication flow completes",
		"All source templates in this reference are written in English",
		"Translate and localize every user-facing message",
		"The official Chinese name of ShengSuanYun is `胜算云`",
		"LoomLoom does not yet have a configured platform and credential. Choose a platform:",
		"This service is jointly supported by CogFoundry and is recommended for users in Mainland China",
		"https://loomloom.shengsuanyun.com/loom/v1",
		"https://console.shengsuanyun.com/user/keys",
		"Authentication after selection: prefer browser login",
		"Authentication after selection: use an API Token directly; browser login is not supported",
		"Ask the user to choose one. Do not authenticate with either platform until they choose",
		"Browser login did not complete. You can configure ShengSuanYun with an API Token instead.",
		"do not immediately output the Token fallback message",
		"https://console.shengsuanyun.com/user/recharge",
		"No CogFoundry credential was detected",
		"https://loomloom.cogfoundry.ai/loom/v1",
		"https://console.cogfoundry.ai/api-keys",
		"https://console.cogfoundry.ai/credits",
		"The current CogFoundry account has insufficient balance",
		"ShengSuanYun and CogFoundry are preset platforms, not a whitelist",
		"treat that as a request to register and activate that Server",
		"loomloom doctor --server <exact-server> --token <exact-token> --output json",
		"Do not use temporary `LOOMLOOM_SERVER=... LOOMLOOM_TOKEN=...` assignments",
		"If Doctor fails, do not persist or switch anything; keep the current configuration active",
		"next_action=persist_token",
		"loomloom server use <name-or-server>",
		"environment_token_set=true",
		"selected profile's `token_env`",
		"Ask whether the user wants the Agent to remove the local environment Token configuration",
		"Only after the user explicitly confirms",
		"loomloom server list --output json",
		"Never assume a fixed environment variable name",
		"without exposing its value or modifying unrelated configuration",
		"already-running parent shell",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("%s missing %q", canonicalSkillReferencesDir, want)
		}
	}
	englishSource := strings.ReplaceAll(text, "胜算云", "")
	for _, r := range englishSource {
		if unicode.Is(unicode.Han, r) {
			t.Fatalf("%s must use English source guidance except for the official brand name 胜算云; found %q", canonicalSkillReferencesDir, r)
		}
	}
	for _, forbidden := range []string{
		"选择后，我会提供对应平台的 Server 和密钥配置指引",
		"http://loomloom.cogfoundry.ai/loom/v1",
		"https://console-dev.cogfoundry",
		"相关地址未知时，我不会自行猜测",
		"CogFoundry 控制台地址必须读取当前环境配置",
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

func TestBundledSkillsRejectExpandedAuthoringConsistently(t *testing.T) {
	root := findRepoRoot(t)
	text := readCanonicalSkillReference(t, root, "template-spec.md")
	for _, want := range []string{
		"Do not author `bindMode=expanded`",
		"TS-TOPOLOGY-001",
		"`multiValue=true` may still represent an ordered content collection",
		"TemplateSpec v1 does not support dynamic-cardinality Step fan-out",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("%s missing %q", canonicalSkillReferencesDir, want)
		}
	}
	for _, forbidden := range []string{
		"`expanded` is only for dynamic multi-value input",
		"`expanded` is for dynamic multi-value input",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("%s still recommends expanded authoring with %q", canonicalSkillReferencesDir, forbidden)
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
