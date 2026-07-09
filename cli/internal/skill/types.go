package skill

import "encoding/json"

const (
	PreviewSchemaVersion          = "loomloom-skill-preview/v1"
	UninstallPreviewSchemaVersion = "loomloom-skill-uninstall-preview/v1"
	MetadataSchemaVersion         = "loomloom-skill/v1"

	SourceMarketListing = "market_listing"
	SourceUserTemplate  = "user_template"

	InputSchemaModeSchema       = "schema"
	InputSchemaModeWorkbookOnly = "workbook_only"
)

type Agent string

const (
	AgentCodex    Agent = "codex"
	AgentClaude   Agent = "claude"
	AgentOpenClaw Agent = "openclaw"
)

type Options struct {
	SourceType string
	Agent      Agent
	OutputDir  string
	DryRun     bool

	ListingID string

	TemplateID        string
	TemplateVersionID string
}

type UninstallOptions struct {
	Dir    string
	DryRun bool
	Force  bool
}

type Preview struct {
	PreviewSchemaVersion string         `json:"previewSchemaVersion"`
	Installable          bool           `json:"installable"`
	SourceType           string         `json:"sourceType"`
	DisplayName          string         `json:"displayName"`
	SkillName            string         `json:"skillName"`
	SkillNameConflict    bool           `json:"skillNameConflict"`
	Agent                string         `json:"agent"`
	OutputDir            string         `json:"outputDir"`
	SourceID             string         `json:"sourceId"`
	ListingID            string         `json:"listingId,omitempty"`
	ListingVersionID     string         `json:"listingVersionId,omitempty"`
	TemplateID           string         `json:"templateId,omitempty"`
	TemplateVersionID    string         `json:"templateVersionId,omitempty"`
	Fields               []InputField   `json:"fields"`
	Pricing              map[string]any `json:"pricing"`
	Warnings             []Issue        `json:"warnings"`
	Errors               []Issue        `json:"errors"`
	BlockingReason       string         `json:"blockingReason"`
	RecommendedAction    string         `json:"recommendedAction"`
	InputSchemaMode      string         `json:"inputSchemaMode"`
}

type Issue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InputField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	ValueType   string `json:"valueType,omitempty"`
	SourceKind  string `json:"sourceKind,omitempty"`
}

type Metadata struct {
	SchemaVersion string `json:"schema_version"`
	GeneratedBy   string `json:"generated_by"`
	SourceType    string `json:"source_type"`
	Agent         string `json:"agent"`
	GeneratedAt   string `json:"generated_at"`
	SkillName     string `json:"skill_name"`
	DisplayName   string `json:"display_name"`
	Description   string `json:"description,omitempty"`
	SourceID      string `json:"source_id"`

	InputSchemaMode     string          `json:"input_schema_mode"`
	InputSchemaSnapshot json.RawMessage `json:"input_schema_snapshot,omitempty"`

	ListingID                 string `json:"listing_id,omitempty"`
	InstalledListingVersionID string `json:"installed_listing_version_id,omitempty"`
	PricingRuleVersion        string `json:"pricing_rule_version,omitempty"`
	TaskFixedFeeT             int64  `json:"task_fixed_fee_t,omitempty"`

	TemplateID        string `json:"template_id,omitempty"`
	TemplateVersionID string `json:"template_version_id,omitempty"`
}

type InstallResult struct {
	Preview
	Installed bool   `json:"installed"`
	Metadata  string `json:"metadataFile,omitempty"`
	SkillFile string `json:"skillFile,omitempty"`
	Trigger   string `json:"triggerExample,omitempty"`
}

type UninstallPreview struct {
	PreviewSchemaVersion string   `json:"previewSchemaVersion"`
	Removable            bool     `json:"removable"`
	Dir                  string   `json:"dir"`
	SkillName            string   `json:"skillName"`
	DisplayName          string   `json:"displayName,omitempty"`
	Agent                string   `json:"agent"`
	SourceType           string   `json:"sourceType,omitempty"`
	SourceID             string   `json:"sourceId,omitempty"`
	Warnings             []Issue  `json:"warnings"`
	Errors               []Issue  `json:"errors"`
	BlockingReason       string   `json:"blockingReason"`
	RecommendedAction    string   `json:"recommendedAction"`
	WillDelete           []string `json:"willDelete"`
	UnexpectedFiles      []string `json:"unexpectedFiles,omitempty"`
}

type UninstallResult struct {
	UninstallPreview
	Uninstalled bool `json:"uninstalled"`
}

type TemplateData struct {
	Metadata     Metadata
	Fields       []InputField
	Instructions []string
	SampleRows   []map[string]any
	Warnings     []Issue
}
