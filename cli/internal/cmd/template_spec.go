package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/client"
	templatespecdocs "github.com/Cogfoundry-ai/loomloom/cli/internal/template_spec_docs"
	"github.com/spf13/cobra"
)

type createUserTemplateResponse struct {
	TemplateID  string    `json:"templateId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   flexInt64 `json:"createdAt"`
}

type saveTemplateVersionResponse struct {
	VersionID      string    `json:"versionId"`
	VersionNumber  flexInt64 `json:"versionNumber"`
	DefinitionHash string    `json:"definitionHash"`
	CreatedAt      flexInt64 `json:"createdAt"`
}

type submitUserTemplateWorkbookResponse struct {
	RunID      string    `json:"runId"`
	Status     string    `json:"status"`
	AcceptedAt flexInt64 `json:"acceptedAt"`
}

func (r *submitUserTemplateWorkbookResponse) UnmarshalJSON(data []byte) error {
	type alias struct {
		RunID             string    `json:"runId"`
		Status            string    `json:"status"`
		AcceptedAt        flexInt64 `json:"acceptedAt"`
		AcceptedAtUnix    flexInt64 `json:"acceptedAtUnix"`
		AcceptedAtUnixAlt flexInt64 `json:"accepted_at_unix"`
	}
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	r.RunID = parsed.RunID
	r.Status = parsed.Status
	r.AcceptedAt = parsed.AcceptedAt
	if r.AcceptedAt == 0 {
		r.AcceptedAt = parsed.AcceptedAtUnix
	}
	if r.AcceptedAt == 0 {
		r.AcceptedAt = parsed.AcceptedAtUnixAlt
	}
	return nil
}

type listModelsResponse struct {
	Models []modelSummary `json:"models"`
}

type modelSummary struct {
	ModelID            string   `json:"modelId"`
	DisplayName        string   `json:"displayName"`
	Provider           string   `json:"provider"`
	ExecutionAdapter   string   `json:"executionAdapter"`
	SupportedStepTypes []string `json:"supportedStepTypes"`
	SupportedAPIs      []string `json:"supportedApis"`
	Available          bool     `json:"available"`
	AvailabilityReason string   `json:"availabilityReason"`
	IsDefault          bool     `json:"isDefault"`
}

type templateSpecMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type templateSpecEnvelope struct {
	Meta          templateSpecMeta         `json:"meta"`
	Steps         []templateSpecStep       `json:"steps"`
	InputSchema   *templateSpecInputSchema `json:"inputSchema"`
	FieldBindings []templateSpecBinding    `json:"fieldBindings"`
	ParamBindings []any                    `json:"paramBindings"`
}

type templateSpecStep struct {
	StepID           string                        `json:"stepId"`
	ExecutionUnit    string                        `json:"executionUnit"`
	UpstreamBindings []templateSpecUpstreamBinding `json:"upstreamBindings"`
}

type templateSpecInputSchema struct {
	Fields     []templateSpecInputField `json:"fields"`
	SampleRows []templateSpecSampleRow  `json:"sampleRows"`
}

type templateSpecSampleRow struct {
	Values map[string]any `json:"values"`
}

type templateSpecInputField struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	ValueType string `json:"valueType"`
}

type templateSpecBinding struct {
	FieldKey string `json:"fieldKey"`
	StepID   string `json:"stepId"`
	ParamKey string `json:"paramKey"`
	BindMode string `json:"bindMode"`
}

type templateSpecUpstreamBinding struct {
	SourceType     string `json:"sourceType"`
	SourceInputKey string `json:"sourceInputKey"`
}

func newTemplateSpecCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template-spec",
		Short: "Author user templates from TemplateSpec JSON",
	}
	cmd.AddCommand(
		newTemplateSpecCheckCmd(opts),
		newTemplateSpecDocsCmd(opts),
		newTemplateSpecModelsCmd(opts),
		newTemplateSpecListCmd(opts),
		newTemplateSpecGetCmd(opts),
		newTemplateSpecVersionsCmd(opts),
		newTemplateSpecCreateCmd(opts),
		newTemplateSpecCreateVersionCmd(opts),
		newTemplateSpecDownloadWorkbookCmd(opts),
		newTemplateSpecValidateWorkbookCmd(opts),
		newTemplateSpecPrecheckWorkbookCmd(opts),
		newTemplateSpecSubmitWorkbookCmd(opts),
		newTemplateSpecPrecheckCmd(opts),
		newTemplateSpecRunCmd(opts),
	)
	return cmd
}

func newTemplateSpecDocsCmd(opts *rootOptions) *cobra.Command {
	var language string
	cmd := &cobra.Command{
		Use:   "docs [topic]",
		Short: "Show the TemplateSpec documentation snapshot shipped with this CLI",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedLanguage, err := templatespecdocs.NormalizeLanguage(strings.TrimSpace(language))
			if err != nil {
				return err
			}
			topic := ""
			if len(args) > 0 {
				topic = strings.TrimSpace(args[0])
			}
			if topic == "" {
				return printTemplateSpecDocsIndex(cmd, opts, normalizedLanguage)
			}
			if topic == "all" {
				return printAllTemplateSpecDocs(cmd, opts, normalizedLanguage)
			}
			return printOneTemplateSpecDoc(cmd, opts, normalizedLanguage, topic)
		},
	}
	cmd.Flags().StringVar(&language, "lang", "en", "Documentation language: en or zh-CN")
	return cmd
}

func printTemplateSpecDocsIndex(cmd *cobra.Command, opts *rootOptions, language string) error {
	topics, err := templatespecdocs.Topics(language)
	if err != nil {
		return err
	}
	manifest, err := templatespecdocs.ReadManifest()
	if err != nil {
		return err
	}
	if opts.output == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"language":          language,
			"languageRevision":  templateSpecLanguageRevision(manifest, language),
			"specRevision":      manifest.SpecRevision,
			"generatedRevision": manifest.GeneratedRevision,
			"owner":             manifest.Owner,
			"topics":            topics,
			"usage":             "loomloom template-spec docs <topic>",
		})
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "TemplateSpec revision: %s\nLanguage: %s\nLanguage revision: %s\nGenerated revision: %s\nOwner: %s\n\nTemplateSpec docs topics:\n", manifest.SpecRevision, language, templateSpecLanguageRevision(manifest, language), manifest.GeneratedRevision, manifest.Owner)
	if err != nil {
		return err
	}
	tw := newTabWriter(cmd.OutOrStdout())
	if _, err := fmt.Fprintln(tw, "topic\tdescription"); err != nil {
		return err
	}
	for _, topic := range topics {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", topic.Name, topic.Description); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "\nUse: loomloom template-spec docs <topic>")
	return err
}

func printOneTemplateSpecDoc(cmd *cobra.Command, opts *rootOptions, language string, topicName string) error {
	topic, content, err := templatespecdocs.Read(language, topicName)
	if err != nil {
		return err
	}
	manifest, err := templatespecdocs.ReadManifest()
	if err != nil {
		return err
	}
	if opts.output == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"language":          language,
			"languageRevision":  templateSpecLanguageRevision(manifest, language),
			"specRevision":      manifest.SpecRevision,
			"generatedRevision": manifest.GeneratedRevision,
			"owner":             manifest.Owner,
			"topic":             topic.Name,
			"filename":          topic.Filename,
			"description":       topic.Description,
			"content":           content,
		})
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "TemplateSpec revision: %s\nLanguage: %s\nLanguage revision: %s\nGenerated revision: %s\nOwner: %s\n\n%s", manifest.SpecRevision, language, templateSpecLanguageRevision(manifest, language), manifest.GeneratedRevision, manifest.Owner, content)
	return err
}

func printAllTemplateSpecDocs(cmd *cobra.Command, opts *rootOptions, language string) error {
	topics, err := templatespecdocs.Topics(language)
	if err != nil {
		return err
	}
	manifest, err := templatespecdocs.ReadManifest()
	if err != nil {
		return err
	}
	docs := make([]map[string]any, 0, len(topics))
	for _, topic := range topics {
		loadedTopic, content, err := templatespecdocs.Read(language, topic.Name)
		if err != nil {
			return err
		}
		docs = append(docs, map[string]any{
			"topic":       loadedTopic.Name,
			"filename":    loadedTopic.Filename,
			"description": loadedTopic.Description,
			"content":     content,
		})
	}
	if opts.output == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"language": language, "languageRevision": templateSpecLanguageRevision(manifest, language), "specRevision": manifest.SpecRevision, "generatedRevision": manifest.GeneratedRevision, "owner": manifest.Owner, "docs": docs})
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "TemplateSpec revision: %s\nLanguage: %s\nLanguage revision: %s\nGenerated revision: %s\nOwner: %s\n\n", manifest.SpecRevision, language, templateSpecLanguageRevision(manifest, language), manifest.GeneratedRevision, manifest.Owner); err != nil {
		return err
	}
	for i, doc := range docs {
		if i > 0 {
			if _, err := fmt.Fprint(cmd.OutOrStdout(), "\n---\n\n"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(cmd.OutOrStdout(), doc["content"]); err != nil {
			return err
		}
	}
	return nil
}

func templateSpecLanguageRevision(manifest templatespecdocs.Manifest, language string) string {
	if language == "zh-CN" {
		return manifest.ChineseRevision
	}
	return manifest.EnglishRevision
}

func newTemplateSpecCheckCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "check <spec-json>",
		Short: "Check that a TemplateSpec JSON file is parseable and has the required top-level shape",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, raw, err := loadTemplateSpecFile(args[0])
			if err != nil {
				return err
			}
			result := map[string]any{
				"valid":       true,
				"name":        spec.Meta.Name,
				"description": spec.Meta.Description,
				"steps":       len(spec.Steps),
				"bindings":    len(spec.FieldBindings) + len(spec.ParamBindings),
				"bytes":       len(raw),
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"valid\nname\t%s\nsteps\t%d\nbindings\t%d\nbytes\t%d\n",
				spec.Meta.Name,
				len(spec.Steps),
				len(spec.FieldBindings)+len(spec.ParamBindings),
				len(raw),
			)
			return err
		},
	}
}

func newTemplateSpecModelsCmd(opts *rootOptions) *cobra.Command {
	var provider string
	var includeUnavailable bool

	cmd := &cobra.Command{
		Use:   "models <step-type>",
		Short: "List executable models available for a TemplateSpec step type",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stepType := strings.TrimSpace(args[0])
			if stepType == "" {
				return errors.New("step-type is required")
			}
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			query := url.Values{}
			query.Set("stepType", stepType)
			query.Set("onlyAvailable", fmt.Sprintf("%t", !includeUnavailable))
			if strings.TrimSpace(provider) != "" {
				query.Set("provider", strings.TrimSpace(provider))
			}

			var resp listModelsResponse
			if err := httpClient.GetJSONWithQuery(ctx, "/models", query, &resp); err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}
			return printTemplateSpecModels(cmd.OutOrStdout(), resp.Models)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Optional model provider filter")
	cmd.Flags().BoolVar(&includeUnavailable, "include-unavailable", false, "Include known but currently unavailable models")
	return cmd
}

func newTemplateSpecCreateCmd(opts *rootOptions) *cobra.Command {
	var name string
	var description string
	var versionNote string

	cmd := &cobra.Command{
		Use:   "create <spec-json>",
		Short: "Create a private user template and save the TemplateSpec JSON as version 1",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, raw, err := loadTemplateSpecFile(args[0])
			if err != nil {
				return err
			}
			effectiveName := firstNonEmpty(name, spec.Meta.Name)
			if effectiveName == "" {
				return errors.New("template name is required; set meta.name or pass --name")
			}
			effectiveDescription := firstNonEmpty(description, spec.Meta.Description)

			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			var createResp createUserTemplateResponse
			if err := httpClient.PostJSON(ctx, "/users/me/templates", map[string]any{
				"name":        effectiveName,
				"description": effectiveDescription,
			}, &createResp); err != nil {
				return err
			}

			versionResp, err := saveTemplateSpecVersion(ctx, httpClient, createResp.TemplateID, raw, versionNote)
			if err != nil {
				return fmt.Errorf("save template version for %s: %w", createResp.TemplateID, err)
			}

			result := map[string]any{
				"templateId":     createResp.TemplateID,
				"name":           createResp.Name,
				"description":    createResp.Description,
				"status":         createResp.Status,
				"versionId":      versionResp.VersionID,
				"versionNumber":  int64(versionResp.VersionNumber),
				"definitionHash": versionResp.DefinitionHash,
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"template_id\t%s\nname\t%s\nversion_id\t%s\nversion_number\t%d\ndefinition_hash\t%s\n",
				createResp.TemplateID,
				createResp.Name,
				versionResp.VersionID,
				int64(versionResp.VersionNumber),
				versionResp.DefinitionHash,
			)
			return err
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Template name override; defaults to meta.name")
	cmd.Flags().StringVar(&description, "description", "", "Template description override; defaults to meta.description")
	cmd.Flags().StringVar(&versionNote, "version-note", "", "Optional note for version 1")
	return cmd
}

func newTemplateSpecCreateVersionCmd(opts *rootOptions) *cobra.Command {
	var versionNote string

	cmd := &cobra.Command{
		Use:   "create-version <template-id> <spec-json>",
		Short: "Create a new immutable version for an existing user template",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := strings.TrimSpace(args[0])
			if templateID == "" {
				return errors.New("template ID is required")
			}
			_, raw, err := loadTemplateSpecFile(args[1])
			if err != nil {
				return err
			}

			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			versionResp, err := saveTemplateSpecVersion(ctx, httpClient, templateID, raw, versionNote)
			if err != nil {
				return fmt.Errorf("save template version for %s: %w", templateID, err)
			}

			result := map[string]any{
				"templateId":     templateID,
				"versionId":      versionResp.VersionID,
				"versionNumber":  int64(versionResp.VersionNumber),
				"definitionHash": versionResp.DefinitionHash,
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"template_id\t%s\nversion_id\t%s\nversion_number\t%d\ndefinition_hash\t%s\n",
				templateID,
				versionResp.VersionID,
				int64(versionResp.VersionNumber),
				versionResp.DefinitionHash,
			)
			return err
		},
	}
	cmd.Flags().StringVar(&versionNote, "version-note", "", "Optional note for the new version")
	return cmd
}

func saveTemplateSpecVersion(ctx context.Context, httpClient *client.Client, templateID string, rawSpec []byte, versionNote string) (saveTemplateVersionResponse, error) {
	var versionResp saveTemplateVersionResponse
	err := httpClient.PostJSON(ctx, "/users/me/templates/"+templateID+"/versions", map[string]any{
		"versionNote":   strings.TrimSpace(versionNote),
		"canonicalSpec": json.RawMessage(rawSpec),
	}, &versionResp)
	return versionResp, err
}

func newTemplateSpecDownloadWorkbookCmd(opts *rootOptions) *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "download-workbook <template-id> <version-id>",
		Short: "Download the Excel workbook generated from a user template version",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			templateID := strings.TrimSpace(args[0])
			versionID := strings.TrimSpace(args[1])
			resp, err := httpClient.GetBinary(ctx, "/users/me/templates/"+templateID+"/versions/"+versionID+"/workbook")
			if err != nil {
				return err
			}
			filename := suggestedDownloadFilename(resp.ContentDisposition)
			if filename == "" {
				filename = templateID + "-" + versionID + ".xlsx"
			}
			targetPath, err := resolveFilePath(outputPath, filepath.Base(filename))
			if err != nil {
				return fmt.Errorf("resolve output file path: %w", err)
			}
			if err := os.WriteFile(targetPath, resp.Body, 0o644); err != nil {
				return fmt.Errorf("write downloaded file: %w", err)
			}
			result := map[string]any{
				"templateId": templateID,
				"versionId":  versionID,
				"path":       targetPath,
				"filename":   filename,
				"size":       len(resp.Body),
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "template_id\t%s\nversion_id\t%s\npath\t%s\nsize\t%d\n", templateID, versionID, targetPath, len(resp.Body))
			return err
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output-file", "f", "", "Output .xlsx path or target directory")
	return cmd
}

func newTemplateSpecValidateWorkbookCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "validate-workbook <template-id> <version-id> <xlsx-path>",
		Short: "Validate a filled user-template workbook",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint := "/users/me/templates/" + strings.TrimSpace(args[0]) + "/versions/" + strings.TrimSpace(args[1]) + ":validateWorkbook"
			resp, err := postUserTemplateWorkbook[validateTemplateFileResponse](cmd.Context(), opts, args[2], endpoint, nil)
			if err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"templateId": args[0],
					"versionId":  args[1],
					"file":       args[2],
					"validation": resp,
				})
			}
			if err := printTemplateFileValidation(cmd.OutOrStdout(), resp); err != nil {
				return err
			}
			if !resp.Valid {
				return templateFileValidationError(resp)
			}
			return nil
		},
	}
}

func newTemplateSpecPrecheckWorkbookCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "precheck-workbook <template-id> <version-id> <xlsx-path>",
		Short: "Estimate cost for a user-template workbook without submitting",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := strings.TrimSpace(args[0])
			versionID := strings.TrimSpace(args[1])
			endpoint := "/users/me/templates/" + url.PathEscape(templateID) + "/versions/" + url.PathEscape(versionID) + ":precheckWorkbook"
			resp, err := postUserTemplateWorkbook[precheckTemplateRowsResponse](cmd.Context(), opts, args[2], endpoint, nil)
			if err != nil {
				return err
			}
			if err := maybeInsufficientBalanceError(opts, resp.BalanceCheck); err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"templateId": templateID,
					"versionId":  versionID,
					"file":       args[2],
					"precheck":   precheckJSONPayload(resp),
				})
			}
			return printPrecheck(cmd.OutOrStdout(), resp)
		},
	}
}

func newTemplateSpecSubmitWorkbookCmd(opts *rootOptions) *cobra.Command {
	var callbackURL string
	var clientRequestID string

	cmd := &cobra.Command{
		Use:   "submit-workbook <template-id> <version-id> <xlsx-path>",
		Short: "Submit a filled user-template workbook as a run",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			validateEndpoint := "/users/me/templates/" + strings.TrimSpace(args[0]) + "/versions/" + strings.TrimSpace(args[1]) + ":validateWorkbook"
			validateResp, err := postUserTemplateWorkbook[validateTemplateFileResponse](cmd.Context(), opts, args[2], validateEndpoint, nil)
			if err != nil {
				return err
			}
			if !validateResp.Valid {
				if opts.output == "json" {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					_ = enc.Encode(map[string]any{
						"templateId": args[0],
						"versionId":  args[1],
						"file":       args[2],
						"validation": validateResp,
					})
				}
				return templateFileValidationError(validateResp)
			}

			crid, generatedRequestID := effectiveClientRequestID(clientRequestID)
			extra := map[string]string{
				"clientRequestId": crid,
			}
			if strings.TrimSpace(callbackURL) != "" {
				extra["callbackUrl"] = strings.TrimSpace(callbackURL)
			}
			submitEndpoint := "/users/me/templates/" + strings.TrimSpace(args[0]) + "/versions/" + strings.TrimSpace(args[1]) + ":runWorkbook"
			printGeneratedClientRequestID(cmd, crid, generatedRequestID)
			submitResp, err := postUserTemplateWorkbook[submitUserTemplateWorkbookResponse](cmd.Context(), opts, args[2], submitEndpoint, extra)
			if err != nil {
				return err
			}
			result := map[string]any{
				"templateId":      args[0],
				"versionId":       args[1],
				"file":            args[2],
				"clientRequestId": crid,
				"runId":           submitResp.RunID,
				"status":          submitResp.Status,
				"acceptedAt":      int64(submitResp.AcceptedAt),
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"template_id\t%s\nversion_id\t%s\nfile\t%s\nrun_id\t%s\nstatus\t%s\naccepted_at\t%s\n",
				args[0],
				args[1],
				args[2],
				submitResp.RunID,
				submitResp.Status,
				formatUnix(int64(submitResp.AcceptedAt)),
			)
			return err
		},
	}
	cmd.Flags().StringVar(&callbackURL, "callback-url", "", "Optional callback URL")
	cmd.Flags().StringVar(&clientRequestID, "client-request-id", "", "Stable idempotency key for retrying the same workbook submission")
	cmd.Flags().StringVar(&clientRequestID, "idempotency-key", "", "Deprecated alias for --client-request-id")
	_ = cmd.Flags().MarkDeprecated("idempotency-key", "use --client-request-id")
	return cmd
}

func loadTemplateSpecFile(path string) (templateSpecEnvelope, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return templateSpecEnvelope{}, nil, fmt.Errorf("read %s: %w", path, err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return templateSpecEnvelope{}, nil, errors.New("template spec file is empty")
	}
	normalized, err := normalizeTemplateSpecJSON(trimmed)
	if err != nil {
		return templateSpecEnvelope{}, nil, err
	}
	var spec templateSpecEnvelope
	if err := json.Unmarshal(normalized, &spec); err != nil {
		return templateSpecEnvelope{}, nil, fmt.Errorf("parse TemplateSpec JSON: %w", err)
	}
	if strings.TrimSpace(spec.Meta.Name) == "" {
		return templateSpecEnvelope{}, nil, errors.New("TemplateSpec meta.name is required")
	}
	if len(spec.Steps) == 0 {
		return templateSpecEnvelope{}, nil, errors.New("TemplateSpec steps must not be empty")
	}
	if spec.InputSchema == nil {
		return templateSpecEnvelope{}, nil, errors.New("TemplateSpec inputSchema is required")
	}
	if err := validateTemplateSpecStructure(spec); err != nil {
		return templateSpecEnvelope{}, nil, err
	}
	if err := validateTemplateSpecAssetBindingContract(spec); err != nil {
		return templateSpecEnvelope{}, nil, err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, normalized); err != nil {
		return templateSpecEnvelope{}, nil, fmt.Errorf("compact TemplateSpec JSON: %w", err)
	}
	return spec, compact.Bytes(), nil
}

var templateSpecStepIDPattern = regexp.MustCompile(`^stp_[0-9a-z]{6,10}$`)

func validateTemplateSpecStructure(spec templateSpecEnvelope) error {
	stepIDs := make(map[string]struct{}, len(spec.Steps))
	for i, step := range spec.Steps {
		stepID := strings.TrimSpace(step.StepID)
		if !templateSpecStepIDPattern.MatchString(stepID) {
			return fmt.Errorf("steps[%d].stepId %q must match stp_<6-10 base36 chars>", i, step.StepID)
		}
		if _, exists := stepIDs[stepID]; exists {
			return fmt.Errorf("steps[%d].stepId %q is duplicated", i, stepID)
		}
		stepIDs[stepID] = struct{}{}
	}
	for i, row := range spec.InputSchema.SampleRows {
		if row.Values == nil {
			return fmt.Errorf("inputSchema.sampleRows[%d] must wrap field values in a values object", i)
		}
	}
	return nil
}

func validateTemplateSpecAssetBindingContract(spec templateSpecEnvelope) error {
	if spec.InputSchema == nil || len(spec.FieldBindings) == 0 {
		return nil
	}
	fieldTypes := make(map[string]string, len(spec.InputSchema.Fields))
	for _, field := range spec.InputSchema.Fields {
		fieldTypes[strings.TrimSpace(field.Key)] = strings.TrimSpace(field.ValueType)
	}
	for i, binding := range spec.FieldBindings {
		if fieldTypes[strings.TrimSpace(binding.FieldKey)] != "text_reference" {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(binding.ParamKey), "prompt") {
			continue
		}
		return fmt.Errorf(
			"fieldBindings[%d].fieldKey %q is text_reference and cannot be bound directly to prompt; bind it with upstreamBindings sourceType=initial_input instead",
			i,
			binding.FieldKey,
		)
	}
	return nil
}

func normalizeTemplateSpecJSON(data []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("parse TemplateSpec JSON: %w", err)
	}
	normalized := normalizeTemplateSpecJSONValue(value)
	out, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("normalize TemplateSpec JSON: %w", err)
	}
	return out, nil
}

func normalizeTemplateSpecJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(typed))
		for _, key := range keys {
			normalizedKey := normalizeTemplateSpecJSONKey(key)
			if _, exists := out[normalizedKey]; exists {
				continue
			}
			out[normalizedKey] = normalizeTemplateSpecJSONValue(typed[key])
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeTemplateSpecJSONValue(item))
		}
		return out
	default:
		return value
	}
}

func normalizeTemplateSpecJSONKey(key string) string {
	if normalized, ok := templateSpecJSONKeyAliases[key]; ok {
		return normalized
	}
	return key
}

var templateSpecJSONKeyAliases = map[string]string{
	"AcceptedMIMETypes":  "acceptedMimeTypes",
	"AllowModelOverride": "allowModelOverride",
	"BindMode":           "bindMode",
	"DefaultModelRef":    "defaultModelRef",
	"DefaultValue":       "defaultValue",
	"DependsOn":          "dependsOn",
	"Description":        "description",
	"DisplayName":        "displayName",
	"DisplayOutputType":  "displayOutputType",
	"EnumValues":         "enumValues",
	"Examples":           "examples",
	"ExecutionUnit":      "executionUnit",
	"FieldBindings":      "fieldBindings",
	"FieldKey":           "fieldKey",
	"Fields":             "fields",
	"Hidden":             "hidden",
	"Hint":               "hint",
	"InputPort":          "inputPort",
	"InputSchema":        "inputSchema",
	"InputSummary":       "inputSummary",
	"Instruction":        "instruction",
	"Instructions":       "instructions",
	"Key":                "key",
	"Kind":               "kind",
	"Label":              "label",
	"Literal":            "literal",
	"MaxValues":          "maxValues",
	"Meta":               "meta",
	"ModelKey":           "modelKey",
	"MultiValue":         "multiValue",
	"Name":               "name",
	"Order":              "order",
	"ParamBindings":      "paramBindings",
	"ParamKey":           "paramKey",
	"Placeholder":        "placeholder",
	"Presentation":       "presentation",
	"PrimaryOutputType":  "primaryOutputType",
	"Required":           "required",
	"SampleRows":         "sampleRows",
	"Scenario":           "scenario",
	"Separator":          "separator",
	"SourceInputKey":     "sourceInputKey",
	"SourceKind":         "sourceKind",
	"SourcePort":         "sourcePort",
	"SourceStepID":       "sourceStepId",
	"SourceType":         "sourceType",
	"StaticParams":       "staticParams",
	"StepID":             "stepId",
	"Steps":              "steps",
	"Tags":               "tags",
	"UpstreamBindings":   "upstreamBindings",
	"ValueType":          "valueType",
	"Widget":             "widget",
}

func postUserTemplateWorkbook[T any](ctx context.Context, opts *rootOptions, workbookPath, endpoint string, extra map[string]string) (T, error) {
	var zero T
	payload, err := workbookPayload(workbookPath, extra)
	if err != nil {
		return zero, err
	}
	httpClient, err := newHTTPClient(opts)
	if err != nil {
		return zero, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	var out T
	if err := httpClient.PostJSON(requestCtx, endpoint, payload, &out); err != nil {
		return zero, err
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func printTemplateSpecModels(w interface {
	Write([]byte) (int, error)
}, models []modelSummary) error {
	if len(models) == 0 {
		_, err := fmt.Fprintln(w, "no models")
		return err
	}
	tw := newTabWriter(w)
	if _, err := fmt.Fprintln(tw, "model_id\tdisplay_name\tprovider\tdefault\tavailable\treason"); err != nil {
		return err
	}
	for _, model := range models {
		reason := model.AvailabilityReason
		if reason == "" {
			reason = "-"
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%t\t%t\t%s\n",
			model.ModelID,
			model.DisplayName,
			model.Provider,
			model.IsDefault,
			model.Available,
			reason,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func newTemplateSpecListCmd(opts *rootOptions) *cobra.Command {
	var (
		status     string
		pageSize   int
		pageOffset int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List my private templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			query := url.Values{}
			if strings.TrimSpace(status) != "" {
				query.Set("status", strings.TrimSpace(status))
			}
			if pageSize > 0 {
				query.Set("pageSize", fmt.Sprintf("%d", pageSize))
			}
			if pageOffset > 0 {
				query.Set("pageOffset", fmt.Sprintf("%d", pageOffset))
			}

			var resp map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/users/me/templates", query, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by template status (default: active)")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Page size")
	cmd.Flags().IntVar(&pageOffset, "page-offset", 0, "Page offset")
	return cmd
}

func newTemplateSpecGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <template-id>",
		Short: "Get one private template with its version list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			path := "/users/me/templates/" + url.PathEscape(strings.TrimSpace(args[0]))
			var resp map[string]any
			if err := httpClient.GetProductJSON(ctx, path, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
}

func newTemplateSpecVersionsCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "versions <template-id>",
		Short: "List versions of a private template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			path := "/users/me/templates/" + url.PathEscape(strings.TrimSpace(args[0])) + "/versions"
			var resp map[string]any
			if err := httpClient.GetProductJSON(ctx, path, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
}

func newTemplateSpecPrecheckCmd(opts *rootOptions) *cobra.Command {
	var (
		versionID   string
		inputFileID string
	)
	cmd := &cobra.Command{
		Use:   "precheck <template-id>",
		Short: "Estimate cost for a private template JSONL input without submitting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trimmedInputFileID, err := normalizeTemplateSpecInputFileID(inputFileID)
			if err != nil {
				return err
			}

			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			templateID := strings.TrimSpace(args[0])
			trimmedVersionID := strings.TrimSpace(versionID)
			payload := map[string]any{
				"versionId":   trimmedVersionID,
				"inputFileId": trimmedInputFileID,
			}

			path := "/users/me/templates/" + url.PathEscape(templateID) + ":precheck"
			var resp precheckTemplateRowsResponse
			if err := httpClient.PostProductJSON(ctx, path, payload, &resp); err != nil {
				return err
			}
			if err := maybeInsufficientBalanceError(opts, resp.BalanceCheck); err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"templateId":  templateID,
					"versionId":   trimmedVersionID,
					"inputFileId": trimmedInputFileID,
					"precheck":    precheckJSONPayload(resp),
				})
			}
			return printPrecheck(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&versionID, "version-id", "", "Template version ID to precheck")
	cmd.Flags().StringVar(&inputFileID, "input-file-id", "", "Execution input fileId returned by orchestrationInputs:upload (not inputAssets:upload)")
	_ = cmd.MarkFlagRequired("version-id")
	_ = cmd.MarkFlagRequired("input-file-id")
	return cmd
}

func newTemplateSpecRunCmd(opts *rootOptions) *cobra.Command {
	var (
		versionID       string
		inputFileID     string
		clientRequestID string
		callbackURL     string
	)
	cmd := &cobra.Command{
		Use:   "run <template-id>",
		Short: "Submit a run for a private template version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trimmedInputFileID, err := normalizeTemplateSpecInputFileID(inputFileID)
			if err != nil {
				return err
			}

			crid, generatedRequestID := effectiveClientRequestID(clientRequestID)

			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			payload := map[string]any{
				"versionId":       strings.TrimSpace(versionID),
				"inputFileId":     trimmedInputFileID,
				"clientRequestId": crid,
			}
			if strings.TrimSpace(callbackURL) != "" {
				payload["callbackUrl"] = strings.TrimSpace(callbackURL)
			}

			path := "/users/me/templates/" + url.PathEscape(strings.TrimSpace(args[0])) + ":run"
			printGeneratedClientRequestID(cmd, crid, generatedRequestID)
			var resp map[string]any
			if err := httpClient.PostProductJSON(ctx, path, payload, &resp); err != nil {
				return err
			}
			opts.debugf(
				"private template run: submitted template_id=%s version_id=%s input_file_id=%s run_id=%s",
				strings.TrimSpace(args[0]),
				strings.TrimSpace(versionID),
				trimmedInputFileID,
				stringMapValue(resp, "runId"),
			)
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&versionID, "version-id", "", "Template version ID to run")
	cmd.Flags().StringVar(&inputFileID, "input-file-id", "", "Execution input fileId returned by orchestrationInputs:upload (not inputAssets:upload)")
	cmd.Flags().StringVar(&clientRequestID, "client-request-id", "", "Idempotency key; auto-generated if omitted")
	cmd.Flags().StringVar(&callbackURL, "callback-url", "", "Optional callback URL")
	_ = cmd.MarkFlagRequired("version-id")
	_ = cmd.MarkFlagRequired("input-file-id")
	return cmd
}

func normalizeTemplateSpecInputFileID(inputFileID string) (string, error) {
	trimmedInputFileID := strings.TrimSpace(inputFileID)
	if strings.HasPrefix(strings.ToLower(trimmedInputFileID), "ia_") {
		return "", fmt.Errorf("--input-file-id requires the fileId returned by orchestrationInputs:upload; inputAssets:upload returns an inputAssetId (%q) that cannot be used to run a template", trimmedInputFileID)
	}
	return trimmedInputFileID, nil
}
