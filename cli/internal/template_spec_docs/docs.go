package templatespecdocs

import (
	"embed"
	"encoding/json"
	"fmt"
)

// FS contains the TemplateSpec documentation snapshot shipped with the CLI.
//
//go:embed generated
var FS embed.FS

type Manifest struct {
	SpecRevision      string `json:"source_revision"`
	GeneratedRevision string `json:"generated_revision"`
	EnglishRevision   string `json:"english_revision"`
	ChineseRevision   string `json:"chinese_revision"`
	Owner             string `json:"owner"`
}

type Topic struct {
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	Description string `json:"description"`
}

var topicsByLanguage = map[string][]Topic{
	"en": {
		{Name: "spec", Filename: "reference/template-syntax.md", Description: "Complete TemplateSpec syntax reference"},
		{Name: "authoring", Filename: "get-started/quickstart.md", Description: "Quickstart for template authors"},
		{Name: "examples", Filename: "examples/README.md", Description: "Executable example index"},
		{Name: "conversation", Filename: "get-started/understand-template-spec.md", Description: "Compatibility alias; agent protocol lives in the installed Skill"},
		{Name: "metadata", Filename: "reference/metadata.md", Description: "Template metadata fields and output declaration"},
		{Name: "inputs", Filename: "reference/input-schema.md", Description: "Input schema and field definitions"},
		{Name: "steps", Filename: "reference/steps.md", Description: "Step fields and DAG topology rules"},
		{Name: "bindings", Filename: "reference/bindings.md", Description: "Input ports and binding rules"},
		{Name: "execution-units", Filename: "reference/execution-units.md", Description: "Execution-unit input ports and output types"},
	},
	"zh-CN": {
		{Name: "spec", Filename: "reference/template-syntax.md", Description: "完整 TemplateSpec 语法参考"},
		{Name: "authoring", Filename: "get-started/quickstart.md", Description: "模板作者快速开始"},
		{Name: "examples", Filename: "examples/README.md", Description: "可执行示例索引"},
		{Name: "conversation", Filename: "get-started/understand-template-spec.md", Description: "兼容入口；Agent 协议位于已安装 Skill"},
		{Name: "metadata", Filename: "reference/metadata.md", Description: "模板元数据和输出声明"},
		{Name: "inputs", Filename: "reference/input-schema.md", Description: "输入 Schema 和字段定义"},
		{Name: "steps", Filename: "reference/steps.md", Description: "步骤字段和 DAG 拓扑规则"},
		{Name: "bindings", Filename: "reference/bindings.md", Description: "输入端口与 Binding 规则"},
		{Name: "execution-units", Filename: "reference/execution-units.md", Description: "执行单元输入端口和输出类型"},
	},
}

func NormalizeLanguage(language string) (string, error) {
	switch language {
	case "", "en":
		return "en", nil
	case "zh-CN":
		return "zh-CN", nil
	default:
		return "", fmt.Errorf("unsupported TemplateSpec docs language %q; use en or zh-CN", language)
	}
}

func Topics(language string) ([]Topic, error) {
	language, err := NormalizeLanguage(language)
	if err != nil {
		return nil, err
	}
	out := append([]Topic(nil), topicsByLanguage[language]...)
	return out, nil
}

func ReadManifest() (Manifest, error) {
	content, err := FS.ReadFile("generated/manifest.json")
	if err != nil {
		return Manifest{}, fmt.Errorf("read TemplateSpec manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode TemplateSpec manifest: %w", err)
	}
	if manifest.SpecRevision == "" || manifest.GeneratedRevision == "" || manifest.EnglishRevision == "" || manifest.ChineseRevision == "" || manifest.Owner == "" {
		return Manifest{}, fmt.Errorf("TemplateSpec manifest is missing source, English, Chinese, generated revision, or owner")
	}
	return manifest, nil
}

func Read(language string, topicName string) (Topic, string, error) {
	language, err := NormalizeLanguage(language)
	if err != nil {
		return Topic{}, "", err
	}
	for _, topic := range topicsByLanguage[language] {
		if topic.Name != topicName {
			continue
		}
		content, err := FS.ReadFile("generated/" + language + "/" + topic.Filename)
		if err != nil {
			return Topic{}, "", fmt.Errorf("read TemplateSpec docs %q: %w", topicName, err)
		}
		return topic, string(content), nil
	}
	return Topic{}, "", fmt.Errorf("unknown TemplateSpec docs topic %q", topicName)
}
