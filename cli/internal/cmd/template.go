package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type listTemplatesResponse struct {
	Templates []templateSummary `json:"templates"`
}

func (r *listTemplatesResponse) UnmarshalJSON(data []byte) error {
	type alias struct {
		Templates []templateSummary `json:"templates"`
		Items     []templateSummary `json:"items"`
	}
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	r.Templates = parsed.Templates
	if len(r.Templates) == 0 {
		r.Templates = parsed.Items
	}
	return nil
}

type templateSummary struct {
	TemplateID   string `json:"templateId"`
	Name         string `json:"name"`
	Scenario     string `json:"scenario"`
	InputSummary string `json:"inputSummary"`
	OutputType   string `json:"outputType"`
	Version      string `json:"version"`
}

type templateSchemaResponse struct {
	TemplateID   string                 `json:"templateId"`
	Version      string                 `json:"version"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Scenario     string                 `json:"scenario"`
	InputSummary string                 `json:"inputSummary"`
	OutputType   string                 `json:"outputType"`
	Fields       []templateFieldSchema  `json:"fields"`
	Columns      []templateColumnSchema `json:"columns"`
	Instructions []string               `json:"instructions"`
	SampleRows   []templateSampleRow    `json:"sampleRows"`
}

type templateFieldSchema struct {
	Key                string   `json:"key"`
	Label              string   `json:"label"`
	Required           bool     `json:"required"`
	Type               string   `json:"type"`
	EnumValues         []string `json:"enumValues"`
	AcceptedMimeTypes  []string `json:"acceptedMimeTypes"`
	AcceptedInputKinds []string `json:"acceptedInputKinds"`
	InputHint          string   `json:"inputHint"`
	Examples           []string `json:"examples"`
	BusinessHint       string   `json:"businessHint"`
	MultiValue         bool     `json:"multiValue"`
	Scope              string   `json:"scope"`
}

type templateColumnSchema struct {
	FieldKey    string `json:"fieldKey"`
	HeaderLabel string `json:"headerLabel"`
	Order       int    `json:"order"`
}

type templateSampleRow struct {
	Values map[string]string `json:"values"`
}

type validateTemplateFileResponse struct {
	Valid      bool                 `json:"valid"`
	FileErrors []string             `json:"fileErrors"`
	RowErrors  []rowValidationError `json:"rowErrors"`
}

func (r *validateTemplateFileResponse) UnmarshalJSON(data []byte) error {
	type alias struct {
		Valid         bool                 `json:"valid"`
		FileErrors    []string             `json:"fileErrors"`
		FileErrorsAlt []string             `json:"file_errors"`
		RowErrors     []rowValidationError `json:"rowErrors"`
		RowErrorsAlt  []rowValidationError `json:"row_errors"`
	}
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	r.Valid = parsed.Valid
	r.FileErrors = parsed.FileErrors
	if len(r.FileErrors) == 0 {
		r.FileErrors = parsed.FileErrorsAlt
	}
	r.RowErrors = parsed.RowErrors
	if len(r.RowErrors) == 0 {
		r.RowErrors = parsed.RowErrorsAlt
	}
	return nil
}

type submitTemplateFileResponse struct {
	RunID      string    `json:"runId"`
	Status     string    `json:"status"`
	AcceptedAt flexInt64 `json:"acceptedAt"`
}

func (r *submitTemplateFileResponse) UnmarshalJSON(data []byte) error {
	type alias struct {
		RunID             string    `json:"runId"`
		RunIDAlt          string    `json:"run_id"`
		Status            string    `json:"status"`
		AcceptedAt        flexInt64 `json:"acceptedAt"`
		AcceptedAtAlt     flexInt64 `json:"accepted_at"`
		AcceptedAtUnix    flexInt64 `json:"acceptedAtUnix"`
		AcceptedAtUnixAlt flexInt64 `json:"accepted_at_unix"`
	}
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	r.RunID = parsed.RunID
	if r.RunID == "" {
		r.RunID = parsed.RunIDAlt
	}
	r.Status = parsed.Status
	r.AcceptedAt = parsed.AcceptedAt
	if r.AcceptedAt == 0 {
		r.AcceptedAt = parsed.AcceptedAtAlt
	}
	if r.AcceptedAt == 0 {
		r.AcceptedAt = parsed.AcceptedAtUnix
	}
	if r.AcceptedAt == 0 {
		r.AcceptedAt = parsed.AcceptedAtUnixAlt
	}
	return nil
}

func newTemplateCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Discover hosted LoomLoom templates",
	}

	cmd.AddCommand(
		newTemplateListCmd(opts),
		newTemplateSchemaCmd(opts),
		newTemplateDownloadCmd(opts),
		newTemplateBackfillResultsCmd(opts),
		newTemplateValidateFileCmd(opts),
		newTemplatePrecheckFileCmd(opts),
		newTemplateSubmitFileCmd(opts),
	)
	return cmd
}

func newTemplateListCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List published templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			var resp listTemplatesResponse
			if err := httpClient.GetJSON(ctx, "/officialTemplates", &resp); err != nil {
				return err
			}

			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}

			if len(resp.Templates) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no templates")
				return nil
			}
			for _, tmpl := range resp.Templates {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", tmpl.TemplateID, tmpl.Name, tmpl.OutputType)
			}
			return nil
		},
	}
}

func newTemplateSchemaCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "schema <template-id>",
		Short: "Inspect one template schema",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			var resp templateSchemaResponse
			path := "/officialTemplates/" + strings.TrimSpace(args[0]) + "/schema"
			if err := httpClient.GetJSON(ctx, path, &resp); err != nil {
				return err
			}

			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "template: %s\n", resp.TemplateID)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "name: %s\n", resp.Name)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "output: %s\n", resp.OutputType)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "scenario: %s\n", resp.Scenario)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "input: %s\n", resp.InputSummary)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "description: %s\n", resp.Description)
			if len(resp.Fields) > 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "fields:")
				for _, field := range resp.Fields {
					required := "optional"
					if field.Required {
						required = "required"
					}
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "- %s (%s) [%s, %s]\n", field.Label, field.Key, field.Type, required)
					if field.InputHint != "" {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  input: %s\n", field.InputHint)
					}
					if field.BusinessHint != "" {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  usage: %s\n", field.BusinessHint)
					}
					if len(field.AcceptedInputKinds) > 0 {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  accepted: %s\n", strings.Join(field.AcceptedInputKinds, ", "))
					}
					if len(field.EnumValues) > 0 {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  enum: %s\n", strings.Join(field.EnumValues, ", "))
					}
					if len(field.AcceptedMimeTypes) > 0 {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  mime: %s\n", strings.Join(field.AcceptedMimeTypes, ", "))
					}
					if len(field.Examples) > 0 {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  examples: %s\n", strings.Join(field.Examples, " | "))
					}
				}
			}
			if len(resp.Columns) > 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "columns:")
				for _, column := range resp.Columns {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "- %s (%s)\n", column.HeaderLabel, column.FieldKey)
				}
			}
			if len(resp.Instructions) > 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "instructions:")
				for _, instruction := range resp.Instructions {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", instruction)
				}
			}
			return nil
		},
	}
}

func newTemplateDownloadCmd(opts *rootOptions) *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "download <template-id>",
		Short: "Download the official template Excel workbook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			resp, err := httpClient.GetBinary(ctx, "/officialTemplates/"+strings.TrimSpace(args[0])+"/workbook")
			if err != nil {
				return err
			}

			filename := suggestedDownloadFilename(resp.ContentDisposition)
			if filename == "" {
				filename = strings.TrimSpace(args[0]) + ".xlsx"
			}
			targetPath, err := resolveFilePath(outputPath, filename)
			if err != nil {
				return fmt.Errorf("resolve output file path: %w", err)
			}
			if err := os.WriteFile(targetPath, resp.Body, 0o644); err != nil {
				return fmt.Errorf("write downloaded file: %w", err)
			}
			opts.debugf("official template workbook: downloaded template_id=%s filename=%s size_bytes=%d", strings.TrimSpace(args[0]), filepath.Base(targetPath), len(resp.Body))

			result := map[string]any{
				"templateId": args[0],
				"path":       targetPath,
				"filename":   filename,
				"size":       len(resp.Body),
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "template_id\t%s\npath\t%s\nsize\t%d\n", args[0], targetPath, len(resp.Body))
			return err
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output-file", "f", "", "Output .xlsx path or target directory")
	return cmd
}

func newTemplateValidateFileCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "validate-file <template-id> <xlsx-path>",
		Short: "Validate one filled official template Excel workbook",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			payload, err := workbookPayload(args[1], nil)
			if err != nil {
				return err
			}

			var resp validateTemplateFileResponse
			path := "/officialTemplates/" + strings.TrimSpace(args[0]) + ":validateWorkbook"
			if err := httpClient.PostJSON(ctx, path, payload, &resp); err != nil {
				return err
			}

			result := map[string]any{
				"templateId": args[0],
				"file":       args[1],
				"validation": resp,
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
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

func newTemplatePrecheckFileCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "precheck-file <template-id> <xlsx-path>",
		Short: "Estimate cost for an official template workbook without submitting",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			payload, err := workbookPayload(args[1], nil)
			if err != nil {
				return err
			}

			var resp map[string]any
			path := "/officialTemplates/" + strings.TrimSpace(args[0]) + ":precheckWorkbook"
			if err := httpClient.PostJSON(ctx, path, payload, &resp); err != nil {
				return err
			}
			if err := maybeMapInsufficientBalanceError(opts, resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
}

func newTemplateSubmitFileCmd(opts *rootOptions) *cobra.Command {
	var (
		callbackURL     string
		clientRequestID string
	)

	cmd := &cobra.Command{
		Use:   "submit-file <template-id> <xlsx-path>",
		Short: "Submit one filled official template Excel workbook as a run",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			payload, err := workbookPayload(args[1], nil)
			if err != nil {
				return err
			}

			var validateResp validateTemplateFileResponse
			validatePath := "/officialTemplates/" + strings.TrimSpace(args[0]) + ":validateWorkbook"
			if err := httpClient.PostJSON(ctx, validatePath, payload, &validateResp); err != nil {
				return err
			}
			if !validateResp.Valid {
				if opts.output == "json" {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					_ = enc.Encode(map[string]any{
						"templateId": args[0],
						"file":       args[1],
						"validation": validateResp,
					})
				}
				return templateFileValidationError(validateResp)
			}

			if strings.TrimSpace(callbackURL) != "" {
				payload["callbackUrl"] = strings.TrimSpace(callbackURL)
			}
			requestID, generatedRequestID := effectiveClientRequestID(clientRequestID)
			payload["clientRequestId"] = requestID
			printGeneratedClientRequestID(cmd, requestID, generatedRequestID)

			var submitResp submitTemplateFileResponse
			submitPath := "/officialTemplates/" + strings.TrimSpace(args[0]) + ":runWorkbook"
			if err := httpClient.PostJSON(ctx, submitPath, payload, &submitResp); err != nil {
				return err
			}

			result := map[string]any{
				"templateId":      args[0],
				"file":            args[1],
				"clientRequestId": requestID,
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
				"template_id\t%s\nfile\t%s\nrun_id\t%s\nstatus\t%s\naccepted_at\t%s\n",
				args[0],
				args[1],
				submitResp.RunID,
				submitResp.Status,
				formatUnix(int64(submitResp.AcceptedAt)),
			)
			return err
		},
	}
	cmd.Flags().StringVar(&callbackURL, "callback-url", "", "Optional callback URL")
	cmd.Flags().StringVar(&clientRequestID, "client-request-id", "", "Stable idempotency key for retrying the same workbook submission")
	return cmd
}

func suggestedDownloadFilename(contentDisposition string) string {
	if strings.TrimSpace(contentDisposition) == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(contentDisposition)
	if err != nil {
		return ""
	}
	filename := strings.TrimSpace(params["filename"])
	if filename == "" {
		return ""
	}
	return filepath.Base(filename)
}

func workbookPayload(workbookPath string, extra map[string]string) (map[string]any, error) {
	content, err := os.ReadFile(workbookPath)
	if err != nil {
		return nil, fmt.Errorf("read workbook: %w", err)
	}
	payload := map[string]any{
		"filename": filepath.Base(workbookPath),
		"content":  content,
	}
	for key, value := range extra {
		payload[key] = value
	}
	return payload, nil
}
