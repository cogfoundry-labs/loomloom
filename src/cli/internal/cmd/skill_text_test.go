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
	"agent-guidance/loomloom",
}

const canonicalSkillReferencesDir = "agent-guidance/loomloom/references"

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

func TestBundledSkillDocumentsUnifiedInstallation(t *testing.T) {
	root := findRepoRoot(t)
	for _, rel := range bundledSkillDirs {
		text, err := os.ReadFile(filepath.Join(root, rel, "SKILL.md"))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, want := range []string{
			"Use the distributed `skills/loomloom` directory as the Skill source",
			"Determine the current Agent's supported Skill root from its runtime configuration or official conventions",
			"Use `<agent-skill-root>/loomloom` as the complete destination",
			"`--skill-dir` on macOS/Linux or `-SkillDir` on Windows",
			"Do not guess the Skill root or fall back to another Agent's directory",
			"If it is unknown, ask the user for it",
			"Verify that `<agent-skill-root>/loomloom/SKILL.md` exists after installation",
		} {
			if !strings.Contains(string(text), want) {
				t.Errorf("%s missing unified installation rule %q", rel, want)
			}
		}
	}
}

func TestTemplateSpecArtifactSequenceFixtureContract(t *testing.T) {
	root := findRepoRoot(t)
	caseData, err := os.ReadFile(filepath.Join(root, "src/cli/internal/cmd/testdata/template-spec-authoring/uploaded-text.json"))
	if err != nil {
		t.Fatalf("read authoring case: %v", err)
	}
	var authoringCase struct {
		Request  string `json:"request"`
		Fixture  string `json:"fixture"`
		Expected struct {
			InputKey         string `json:"inputKey"`
			InputKind        string `json:"inputKind"`
			AcceptedMimeType string `json:"acceptedMimeType"`
			TargetPort       string `json:"targetPort"`
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
			InputBindings map[string]struct {
				Sequence struct {
					Items []struct {
						Source struct {
							Source   string `json:"source"`
							InputKey string `json:"inputKey"`
						} `json:"source"`
					} `json:"items"`
				} `json:"sequence"`
			} `json:"inputBindings"`
		} `json:"steps"`
		TemplateInputs map[string]struct {
			Kind              string   `json:"kind"`
			AcceptedMimeTypes []string `json:"acceptedMimeTypes"`
		} `json:"templateInputs"`
	}
	if err := json.Unmarshal(specData, &spec); err != nil {
		t.Fatalf("decode fixture spec: %v", err)
	}
	input, ok := spec.TemplateInputs[authoringCase.Expected.InputKey]
	if !ok {
		t.Fatalf("fixture missing template input %q", authoringCase.Expected.InputKey)
	}
	if input.Kind != authoringCase.Expected.InputKind {
		t.Fatalf("input kind = %q, want %q", input.Kind, authoringCase.Expected.InputKind)
	}
	if len(input.AcceptedMimeTypes) != 1 || input.AcceptedMimeTypes[0] != authoringCase.Expected.AcceptedMimeType {
		t.Fatalf("acceptedMimeTypes = %v, want [%q]", input.AcceptedMimeTypes, authoringCase.Expected.AcceptedMimeType)
	}
	if len(spec.Steps) != 1 {
		t.Fatalf("fixture steps = %d, want 1", len(spec.Steps))
	}
	binding, ok := spec.Steps[0].InputBindings[authoringCase.Expected.TargetPort]
	if !ok || len(binding.Sequence.Items) < 2 {
		t.Fatalf("fixture missing sequence binding for target port %q", authoringCase.Expected.TargetPort)
	}
	source := binding.Sequence.Items[1].Source
	if source.Source != authoringCase.Expected.SourceType || source.InputKey != authoringCase.Expected.InputKey {
		t.Fatalf("sequence source = %#v, want source=%q inputKey=%q", source, authoringCase.Expected.SourceType, authoringCase.Expected.InputKey)
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
		"Browser login is available for both preset platforms, ShengSuanYun and CogFoundry",
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
		"After the user selects ShengSuanYun or CogFoundry",
		"loomloom login",
		"After the user selects a custom platform",
		"After either authentication flow completes",
		"All source templates in this reference are written in English",
		"Translate and localize every user-facing message",
		"The official Chinese name of ShengSuanYun is `胜算云`",
		"LoomLoom does not yet have a configured platform and credential. Choose a platform:",
		"This service is jointly supported by CogFoundry and is recommended for users in Mainland China",
		"https://loomloom.shengsuanyun.com/loom/v1",
		"https://console.shengsuanyun.com/user/keys",
		"Authentication after selection: prefer browser login",
		"Browser login did not complete. You can configure CogFoundry with an API Token instead.",
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
		"without exposing their values or modifying unrelated configuration",
		"Installer Uninstall Credential Cleanup",
		"environment token cleanup required: LOOMLOOM_TOKEN_<PROFILE>",
		"Ask whether the user wants the Agent to remove those variables",
		"Reported names are cleanup candidates and may not currently be configured",
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

func TestBundledSkillsUseTemplateSpecV2Bindings(t *testing.T) {
	root := findRepoRoot(t)
	text := readCanonicalSkillReference(t, root, "template-spec.md")
	for _, want := range []string{
		"Author only `template-spec/v2`; v1 is historical and read-only",
		"top-level `templateInputs` map",
		"`executionBinding.kind=fixedModelContract`",
		"both `dependsOn` and a `stepOutput` source",
		"Use `sequence` for one ordered heterogeneous multimodal value",
		"Use `merge` for homogeneous Artifact collections",
		"TemplateSpec v2 does not provide dynamic-cardinality Step fan-out",
		"An empty result from `template-spec contracts <text-model-id>`",
		"`executionBinding.kind=capabilityProfile`",
		"must never be reported as blocking such a workflow",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("%s missing %q", canonicalSkillReferencesDir, want)
		}
	}
	for _, forbidden := range []string{
		"Connect steps with `dependsOn` and `upstreamBindings`",
		"`allowModelOverride=true`",
		"`inputSchema.sampleRows",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("%s still recommends retired v1 authoring with %q", canonicalSkillReferencesDir, forbidden)
		}
	}
}

func TestBundledSkillGuidesLegacyTemplateSpecV1Upgrade(t *testing.T) {
	root := findRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agent-guidance", "loomloom", "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"Historical v1 TemplateVersions remain readable and runnable",
		"used to create a new template or save a new version",
		"never overwrite or claim to repair the historical v1 version",
		"Before changing, copying, or appending a version to an existing private",
		"loomloom template-spec get <template-id> --output json",
		"loomloom template-spec versions <template-id> --output json",
		"inspect that version's returned `specVersion`",
		"do not submit its historical JSON to `create-version`",
		"Do not force this upgrade",
		"only wants to run an existing v1 version",
		"loomloom template-spec docs spec --lang zh-CN",
		"loomloom capability resolve --input <modality> --output-modality <modality> --output json",
		"loomloom template-spec authoring-context --output json",
		"loomloom template-spec contracts <model-id> --output json",
		"promise a lossless or automatic v1-to-v2 conversion",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("agent-guidance/loomloom/SKILL.md missing %q", want)
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

func TestBundledSkillsDocumentLoginTimeoutBoundaries(t *testing.T) {
	root := findRepoRoot(t)
	setup := readCanonicalSkillReference(t, root, "setup.md")
	cli := readCanonicalSkillReference(t, root, "cli.md")

	for _, want := range []string{
		"loomloom login --login-timeout 10m",
		"loomloom login --no-browser --login-timeout 10m",
		"`--login-timeout` controls only the human authorization window",
		"the global `--timeout` flag controls individual HTTP requests",
	} {
		if !strings.Contains(setup, want) {
			t.Fatalf("%s/setup.md missing %q", canonicalSkillReferencesDir, want)
		}
	}

	for _, want := range []string{
		"loomloom login [--no-browser] [--login-timeout <duration>]",
		"The global `--timeout` flag remains the per-request HTTP timeout",
	} {
		if !strings.Contains(cli, want) {
			t.Fatalf("%s/cli.md missing %q", canonicalSkillReferencesDir, want)
		}
	}
}

func TestBundledSkillsDocumentDistributionFallbackAndAPIReferences(t *testing.T) {
	root := findRepoRoot(t)
	setup := readCanonicalSkillReference(t, root, "setup.md")

	for _, want := range []string{
		"Use GitHub as the default distribution source",
		"If the user explicitly requests Gitee, use the Gitee installer directly",
		"ask whether they want to retry through Gitee",
		"Do not switch to Gitee until the user agrees",
		"must not select or change the LoomLoom platform, Server, or credentials",
		"https://gitee.com/cogfoundry/loomloom/raw/main/scripts/install-gitee.sh",
		"-Source gitee",
	} {
		if !strings.Contains(setup, want) {
			t.Fatalf("%s/setup.md missing %q", canonicalSkillReferencesDir, want)
		}
	}

	for _, rel := range bundledSkillDirs {
		text, err := os.ReadFile(filepath.Join(root, rel, "SKILL.md"))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, want := range []string{
			"Use platform-specific official API documentation when needed",
			"successful `loomloom doctor --output json` result",
			"https://lean.shengsuanyun.com/apidocs/loomloom/api",
			"authoritative source for ShengSuanYun API contracts",
			"CogFoundry API documentation is not yet publicly available",
			"Do not use ShengSuanYun-specific API contracts to infer CogFoundry behavior",
		} {
			if !strings.Contains(string(text), want) {
				t.Fatalf("%s/SKILL.md missing %q", rel, want)
			}
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
		if _, err := os.Stat(filepath.Join(dir, "agent-guidance", "loomloom")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root with agent-guidance/loomloom directory not found")
		}
		dir = parent
	}
}
