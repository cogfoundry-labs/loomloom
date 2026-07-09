package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/client"
)

type Exporter struct {
	client *client.Client
	now    func() time.Time
}

func NewExporter(httpClient *client.Client) *Exporter {
	return &Exporter{
		client: httpClient,
		now:    time.Now,
	}
}

func (e *Exporter) Install(ctx context.Context, opts Options) (*InstallResult, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	data, err := e.load(ctx, opts)
	if err != nil {
		return nil, err
	}
	preview := previewFromData(opts, data)
	if err := checkOutputDirName(opts.OutputDir, preview.SkillName); err != nil {
		preview.Installable = false
		preview.BlockingReason = "output_dir_name_mismatch"
		preview.RecommendedAction = fmt.Sprintf("Use an output directory whose final path segment is %q.", preview.SkillName)
		preview.Errors = append(preview.Errors, Issue{
			Code:    "output_dir_name_mismatch",
			Message: err.Error(),
		})
		if opts.DryRun {
			return &InstallResult{Preview: preview}, nil
		}
		return nil, err
	}
	if err := checkOutputDir(opts.OutputDir, opts.DryRun); err != nil {
		preview.Installable = false
		if errors.Is(err, ErrOutputDirNotEmpty) {
			preview.SkillNameConflict = true
			preview.BlockingReason = "skill_name_conflict"
			preview.RecommendedAction = "Choose an empty output directory or remove the existing generated skill first."
			preview.Errors = append(preview.Errors, Issue{
				Code:    "skill_name_conflict",
				Message: err.Error(),
			})
		} else {
			preview.BlockingReason = "output_dir_unavailable"
			preview.RecommendedAction = "Choose an output directory whose parent exists and is writable."
			preview.Errors = append(preview.Errors, Issue{
				Code:    "output_dir_unavailable",
				Message: err.Error(),
			})
		}
		if opts.DryRun {
			return &InstallResult{Preview: preview}, nil
		}
		return nil, err
	}
	if opts.DryRun {
		return &InstallResult{Preview: preview}, nil
	}

	rendered, err := Render(data)
	if err != nil {
		return nil, err
	}
	if err := writeSkillDir(opts.OutputDir, rendered.SkillMarkdown, rendered.MetadataJSON); err != nil {
		return nil, err
	}
	result := &InstallResult{
		Preview:   preview,
		Installed: true,
		Metadata:  filepath.Join(opts.OutputDir, "loomloom-skill.json"),
		SkillFile: filepath.Join(opts.OutputDir, "SKILL.md"),
		Trigger:   triggerExample(data.Metadata.DisplayName),
	}
	return result, nil
}

func validateOptions(opts Options) error {
	switch opts.Agent {
	case AgentCodex, AgentClaude, AgentOpenClaw:
	default:
		return fmt.Errorf("unsupported --agent %q; use codex, claude, or openclaw", opts.Agent)
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return fmt.Errorf("--output-dir is required")
	}
	switch opts.SourceType {
	case SourceMarketListing:
		if strings.TrimSpace(opts.ListingID) == "" {
			return fmt.Errorf("listing id is required")
		}
	case SourceUserTemplate:
		if strings.TrimSpace(opts.TemplateID) == "" {
			return fmt.Errorf("template id is required")
		}
		if strings.TrimSpace(opts.TemplateVersionID) == "" {
			return fmt.Errorf("template version id is required")
		}
	default:
		return fmt.Errorf("unsupported source type %q", opts.SourceType)
	}
	return nil
}

func checkOutputDirName(outputDir string, skillName string) error {
	base := filepath.Base(filepath.Clean(strings.TrimSpace(outputDir)))
	if base == "." || base == string(filepath.Separator) || base == "" {
		return fmt.Errorf("output directory must end with generated skill name %q", skillName)
	}
	if base != skillName {
		return fmt.Errorf("output directory basename %q must match generated skill name %q", base, skillName)
	}
	return nil
}

func (e *Exporter) load(ctx context.Context, opts Options) (TemplateData, error) {
	switch opts.SourceType {
	case SourceMarketListing:
		return e.loadMarket(ctx, opts)
	case SourceUserTemplate:
		return e.loadUserTemplate(ctx, opts)
	default:
		return TemplateData{}, fmt.Errorf("unsupported source type %q", opts.SourceType)
	}
}

type marketListing struct {
	ID                          string          `json:"id"`
	DisplayName                 string          `json:"displayName"`
	Description                 string          `json:"description"`
	Status                      string          `json:"status"`
	ListingVersionID            string          `json:"listingVersionId"`
	PricingRuleVersion          string          `json:"pricingRuleVersion"`
	TaskFixedFeeT               flexInt64       `json:"taskFixedFeeT"`
	SaleStatus                  string          `json:"saleStatus"`
	ExecutionAvailabilityStatus string          `json:"executionAvailabilityStatus"`
	InputSchemaSnapshot         json.RawMessage `json:"inputSchemaSnapshot"`
}

type publicInputSchema struct {
	SchemaVersion string           `json:"schema_version"`
	Fields        []publicField    `json:"fields"`
	Instructions  []string         `json:"instructions"`
	SampleRows    []map[string]any `json:"sample_rows"`
}

func (s *publicInputSchema) UnmarshalJSON(data []byte) error {
	type alias struct {
		SchemaVersion    string           `json:"schema_version"`
		SchemaVersionAlt string           `json:"schemaVersion"`
		Fields           []publicField    `json:"fields"`
		Instructions     []string         `json:"instructions"`
		SampleRows       []map[string]any `json:"sample_rows"`
		SampleRowsAlt    []map[string]any `json:"sampleRows"`
	}
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	s.SchemaVersion = parsed.SchemaVersion
	if s.SchemaVersion == "" {
		s.SchemaVersion = parsed.SchemaVersionAlt
	}
	s.Fields = parsed.Fields
	s.Instructions = parsed.Instructions
	s.SampleRows = parsed.SampleRows
	if len(s.SampleRows) == 0 {
		s.SampleRows = parsed.SampleRowsAlt
	}
	return nil
}

type publicField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	ValueType   string `json:"value_type"`
	SourceKind  string `json:"source_kind"`
}

func (f *publicField) UnmarshalJSON(data []byte) error {
	type alias struct {
		Key           string `json:"key"`
		Label         string `json:"label"`
		Description   string `json:"description"`
		Required      bool   `json:"required"`
		ValueType     string `json:"value_type"`
		ValueTypeAlt  string `json:"valueType"`
		SourceKind    string `json:"source_kind"`
		SourceKindAlt string `json:"sourceKind"`
	}
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	f.Key = parsed.Key
	f.Label = parsed.Label
	f.Description = parsed.Description
	f.Required = parsed.Required
	f.ValueType = parsed.ValueType
	if f.ValueType == "" {
		f.ValueType = parsed.ValueTypeAlt
	}
	f.SourceKind = parsed.SourceKind
	if f.SourceKind == "" {
		f.SourceKind = parsed.SourceKindAlt
	}
	return nil
}

type flexInt64 int64

func (v *flexInt64) UnmarshalJSON(data []byte) error {
	trimmed := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if trimmed == "" || trimmed == "null" {
		*v = 0
		return nil
	}
	var parsed int64
	if _, err := fmt.Sscan(trimmed, &parsed); err != nil {
		return fmt.Errorf("parse int64 %q: %w", trimmed, err)
	}
	*v = flexInt64(parsed)
	return nil
}

func (e *Exporter) loadMarket(ctx context.Context, opts Options) (TemplateData, error) {
	path := "/marketListings/" + url.PathEscape(strings.TrimSpace(opts.ListingID))
	var listing marketListing
	if err := e.client.GetProductJSON(ctx, path, &listing); err != nil {
		return TemplateData{}, err
	}
	if strings.TrimSpace(listing.ID) == "" {
		return TemplateData{}, fmt.Errorf("market listing id is empty")
	}
	if strings.TrimSpace(listing.DisplayName) == "" {
		return TemplateData{}, fmt.Errorf("market listing displayName is empty")
	}
	if strings.TrimSpace(listing.ListingVersionID) == "" {
		return TemplateData{}, fmt.Errorf("market listing listingVersionId is empty")
	}
	if !marketStatusLooksInstallable(listing) {
		return TemplateData{}, fmt.Errorf("market listing is not currently installable: saleStatus=%s executionAvailabilityStatus=%s status=%s", listing.SaleStatus, listing.ExecutionAvailabilityStatus, listing.Status)
	}
	schema, err := parseInputSchema(listing.InputSchemaSnapshot)
	if err != nil {
		return TemplateData{}, fmt.Errorf("parse inputSchemaSnapshot: %w", err)
	}
	fields := fieldsFromSchema(schema)
	skillName := SkillName(listing.DisplayName, listing.ID)
	return TemplateData{
		Metadata: Metadata{
			SchemaVersion:             MetadataSchemaVersion,
			GeneratedBy:               "loomloom-cli",
			SourceType:                SourceMarketListing,
			Agent:                     string(opts.Agent),
			GeneratedAt:               e.now().UTC().Format(time.RFC3339),
			SkillName:                 skillName,
			DisplayName:               listing.DisplayName,
			Description:               listing.Description,
			SourceID:                  listing.ID,
			InputSchemaMode:           InputSchemaModeSchema,
			InputSchemaSnapshot:       listing.InputSchemaSnapshot,
			ListingID:                 listing.ID,
			InstalledListingVersionID: listing.ListingVersionID,
			PricingRuleVersion:        listing.PricingRuleVersion,
			TaskFixedFeeT:             int64(listing.TaskFixedFeeT),
		},
		Fields:       fields,
		Instructions: schema.Instructions,
		SampleRows:   schema.SampleRows,
	}, nil
}

func marketStatusLooksInstallable(listing marketListing) bool {
	sale := strings.ToLower(strings.TrimSpace(listing.SaleStatus))
	availability := strings.ToLower(strings.TrimSpace(listing.ExecutionAvailabilityStatus))
	status := strings.ToLower(strings.TrimSpace(listing.Status))
	if sale != "" && sale != "listed" {
		return false
	}
	if availability != "" && availability != "available" && availability != "executable" {
		return false
	}
	if status != "" && status != "published" && status != "active" {
		return false
	}
	return true
}

func parseInputSchema(raw json.RawMessage) (publicInputSchema, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == `""` {
		return publicInputSchema{}, fmt.Errorf("inputSchemaSnapshot is empty")
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err == nil {
		raw = json.RawMessage(encoded)
	}
	var schema publicInputSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return publicInputSchema{}, err
	}
	if len(schema.Fields) == 0 {
		return publicInputSchema{}, fmt.Errorf("inputSchemaSnapshot has no fields")
	}
	return schema, nil
}

func fieldsFromSchema(schema publicInputSchema) []InputField {
	fields := make([]InputField, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		fields = append(fields, InputField{
			Key:         field.Key,
			Label:       field.Label,
			Description: field.Description,
			Required:    field.Required,
			ValueType:   field.ValueType,
			SourceKind:  field.SourceKind,
		})
	}
	return fields
}

func (e *Exporter) loadUserTemplate(ctx context.Context, opts Options) (TemplateData, error) {
	templateID := strings.TrimSpace(opts.TemplateID)
	versionID := strings.TrimSpace(opts.TemplateVersionID)
	var tmpl map[string]any
	if err := e.client.GetProductJSON(ctx, "/users/me/templates/"+url.PathEscape(templateID), &tmpl); err != nil {
		return TemplateData{}, err
	}
	var versions map[string]any
	if err := e.client.GetProductJSON(ctx, "/users/me/templates/"+url.PathEscape(templateID)+"/versions", &versions); err != nil {
		return TemplateData{}, err
	}
	if !hasVersionObject(versions, versionID) {
		return TemplateData{}, fmt.Errorf("template version %s was not found for template %s", versionID, templateID)
	}
	displayName := firstString(tmpl, "name", "displayName", "title")
	if displayName == "" {
		displayName = templateID
	}
	description := firstString(tmpl, "description", "summary")

	schemaRaw, fields, instructions, sampleRows := findSchemaForVersion(tmpl, versionID)
	if len(fields) == 0 {
		schemaRaw, fields, instructions, sampleRows = findSchemaForVersion(versions, versionID)
	}
	if len(fields) == 0 {
		var executables map[string]any
		if err := e.client.GetProductJSONWithQuery(ctx, "/users/me/executables", nil, &executables); err == nil {
			schemaRaw, fields, instructions, sampleRows = findExecutableSchema(executables, templateID, versionID)
		}
	}
	mode := InputSchemaModeSchema
	warnings := []Issue(nil)
	if len(fields) == 0 {
		mode = InputSchemaModeWorkbookOnly
		warnings = append(warnings, Issue{
			Code:    "input_schema_workbook_only",
			Message: "Full structured input schema was not available during installation. The generated skill will use a workbook-first flow, and execution cannot continue if the workbook is unavailable at run time.",
		})
	}
	skillName := SkillName(displayName, templateID+"-"+versionID)
	return TemplateData{
		Metadata: Metadata{
			SchemaVersion:       MetadataSchemaVersion,
			GeneratedBy:         "loomloom-cli",
			SourceType:          SourceUserTemplate,
			Agent:               string(opts.Agent),
			GeneratedAt:         e.now().UTC().Format(time.RFC3339),
			SkillName:           skillName,
			DisplayName:         displayName,
			Description:         description,
			SourceID:            templateID + ":" + versionID,
			InputSchemaMode:     mode,
			InputSchemaSnapshot: schemaRaw,
			TemplateID:          templateID,
			TemplateVersionID:   versionID,
		},
		Fields:       fields,
		Instructions: instructions,
		SampleRows:   sampleRows,
		Warnings:     warnings,
	}, nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fmt.Sprint(m[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func containsStringValue(value any, target string) bool {
	target = strings.TrimSpace(target)
	switch v := value.(type) {
	case map[string]any:
		for _, child := range v {
			if containsStringValue(child, target) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if containsStringValue(child, target) {
				return true
			}
		}
	case string:
		return strings.TrimSpace(v) == target
	}
	return false
}

func findExecutableSchema(value any, templateID string, versionID string) (json.RawMessage, []InputField, []string, []map[string]any) {
	var foundRaw json.RawMessage
	var foundFields []InputField
	var foundInstructions []string
	var foundSampleRows []map[string]any
	walk(value, func(m map[string]any) bool {
		if !mapContains(m, templateID) || !mapContains(m, versionID) {
			return true
		}
		raw, fields, instructions, sampleRows := findSchemaForVersion(m, versionID)
		if len(fields) == 0 {
			return true
		}
		foundRaw, foundFields = raw, fields
		foundInstructions, foundSampleRows = instructions, sampleRows
		return false
	})
	return foundRaw, foundFields, foundInstructions, foundSampleRows
}

func mapContains(m map[string]any, needle string) bool {
	for _, value := range m {
		if containsStringValue(value, needle) {
			return true
		}
	}
	return false
}

func findSchemaForVersion(value any, versionID string) (json.RawMessage, []InputField, []string, []map[string]any) {
	versionObject := findVersionObject(value, versionID)
	if versionObject == nil {
		return nil, nil, nil, nil
	}
	return findSchemaInObject(versionObject, versionID)
}

func hasVersionObject(value any, versionID string) bool {
	return findVersionObject(value, versionID) != nil
}

func findVersionObject(value any, versionID string) map[string]any {
	var found map[string]any
	walk(value, func(m map[string]any) bool {
		if mapHasVersionID(m, versionID) {
			found = m
			return false
		}
		return true
	})
	return found
}

func mapHasVersionID(m map[string]any, versionID string) bool {
	for _, key := range []string{"versionId", "versionID", "version_id", "templateVersionId", "templateVersionID", "template_version_id", "id"} {
		if strings.TrimSpace(fmt.Sprint(m[key])) == versionID {
			return true
		}
	}
	return false
}

func mapHasAnyVersionID(m map[string]any) bool {
	for _, key := range []string{"versionId", "versionID", "version_id", "templateVersionId", "templateVersionID", "template_version_id", "id"} {
		if value := strings.TrimSpace(fmt.Sprint(m[key])); value != "" && value != "<nil>" {
			return true
		}
	}
	return false
}

func findSchemaInObject(m map[string]any, versionID string) (json.RawMessage, []InputField, []string, []map[string]any) {
	if raw, fields, instructions, sampleRows := schemaFromObject(m); len(fields) > 0 {
		return raw, fields, instructions, sampleRows
	}
	for _, key := range []string{"canonicalSpec", "canonical_spec", "templateSpec", "template_spec", "spec", "definition"} {
		child, ok := m[key].(map[string]any)
		if !ok {
			continue
		}
		if raw, fields, instructions, sampleRows := schemaFromObject(child); len(fields) > 0 {
			return raw, fields, instructions, sampleRows
		}
	}
	return findSchemaInNestedObject(m, versionID)
}

func schemaFromObject(m map[string]any) (json.RawMessage, []InputField, []string, []map[string]any) {
	for _, key := range []string{"inputSchemaSnapshot", "input_schema_snapshot", "inputSchema", "input_schema"} {
		candidate, ok := m[key]
		if !ok {
			continue
		}
		raw, fields, instructions, sampleRows := schemaFromAny(candidate)
		if len(fields) > 0 {
			return raw, fields, instructions, sampleRows
		}
	}
	return nil, nil, nil, nil
}

func findSchemaInNestedObject(m map[string]any, versionID string) (json.RawMessage, []InputField, []string, []map[string]any) {
	var raw json.RawMessage
	var fields []InputField
	var instructions []string
	var sampleRows []map[string]any
	var visit func(any, bool) bool
	visit = func(value any, isRoot bool) bool {
		switch v := value.(type) {
		case map[string]any:
			if !isRoot && mapHasAnyVersionID(v) && !mapHasVersionID(v, versionID) {
				return true
			}
			candidateRaw, candidateFields, candidateInstructions, candidateSampleRows := schemaFromObject(v)
			if len(candidateFields) > 0 {
				raw, fields = candidateRaw, candidateFields
				instructions, sampleRows = candidateInstructions, candidateSampleRows
				return false
			}
			for _, child := range v {
				if !visit(child, false) {
					return false
				}
			}
		case []any:
			for _, child := range v {
				if !visit(child, false) {
					return false
				}
			}
		}
		return true
	}
	visit(m, true)
	return raw, fields, instructions, sampleRows
}

func walk(value any, visit func(map[string]any) bool) bool {
	switch v := value.(type) {
	case map[string]any:
		if !visit(v) {
			return false
		}
		for _, child := range v {
			if !walk(child, visit) {
				return false
			}
		}
	case []any:
		for _, child := range v {
			if !walk(child, visit) {
				return false
			}
		}
	}
	return true
}

func schemaFromAny(value any) (json.RawMessage, []InputField, []string, []map[string]any) {
	var raw json.RawMessage
	switch v := value.(type) {
	case string:
		raw = json.RawMessage(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, nil, nil, nil
		}
		raw = data
	}
	schema, err := parseInputSchema(raw)
	if err != nil {
		return raw, nil, nil, nil
	}
	return raw, fieldsFromSchema(schema), schema.Instructions, schema.SampleRows
}

func previewFromData(opts Options, data TemplateData) Preview {
	pricing := map[string]any{}
	if data.Metadata.SourceType == SourceMarketListing {
		pricing["pricingRuleVersion"] = data.Metadata.PricingRuleVersion
		pricing["taskFixedFeeT"] = data.Metadata.TaskFixedFeeT
	}
	return Preview{
		PreviewSchemaVersion: PreviewSchemaVersion,
		Installable:          true,
		SourceType:           data.Metadata.SourceType,
		DisplayName:          data.Metadata.DisplayName,
		SkillName:            data.Metadata.SkillName,
		SkillNameConflict:    false,
		Agent:                string(opts.Agent),
		OutputDir:            opts.OutputDir,
		SourceID:             data.Metadata.SourceID,
		ListingID:            data.Metadata.ListingID,
		ListingVersionID:     data.Metadata.InstalledListingVersionID,
		TemplateID:           data.Metadata.TemplateID,
		TemplateVersionID:    data.Metadata.TemplateVersionID,
		Fields:               data.Fields,
		Pricing:              pricing,
		Warnings:             data.Warnings,
		Errors:               []Issue{},
		BlockingReason:       "",
		RecommendedAction:    "",
		InputSchemaMode:      data.Metadata.InputSchemaMode,
	}
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

func SkillName(displayName string, fallback string) string {
	const prefix = "loomloom-"
	slug := slugifySkillNamePart(displayName)
	if slug == "" {
		slug = slugifySkillNamePart(fallback)
	}
	if slug == "" || slug == "loomloom" {
		slug = "skill"
	}
	if !strings.HasPrefix(slug, prefix) {
		slug = prefix + slug
	}
	if len(slug) > 63 {
		slug = strings.Trim(slug[:63], "-")
	}
	if slug == "" || slug == "loomloom" {
		return "loomloom-skill"
	}
	return slug
}

func slugifySkillNamePart(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = nonSlugChars.ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}

func triggerExample(displayName string) string {
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = "this LoomLoom skill"
	}
	return "Use " + name + " for a matching batch task."
}
