package cmd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/client"
	"github.com/spf13/cobra"
)

func newMarketCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "market",
		Short: "LoomLoom Market SkillBot commands",
	}
	cmd.AddCommand(
		newMarketListCmd(opts),
		newMarketShowCmd(opts),
		newMarketQuoteCmd(opts),
		newMarketRunCmd(opts),
		newMarketWorkbookCmd(opts),
		newDeprecatedMarketPublishCmd(opts),
		newDeprecatedMarketRelistCmd(opts),
	)
	return cmd
}

type marketListingsResponse struct {
	Items         []marketListingPublicResponse `json:"items"`
	NextPageToken string                        `json:"nextPageToken,omitempty"`
}

type marketListingPublicResponse struct {
	ID                          string    `json:"id"`
	DisplayName                 string    `json:"displayName"`
	Description                 string    `json:"description"`
	Status                      string    `json:"status"`
	ListingVersionID            string    `json:"listingVersionId"`
	PricingRuleVersion          string    `json:"pricingRuleVersion"`
	TaskFixedFeeT               flexInt64 `json:"taskFixedFeeT"`
	Currency                    string    `json:"currency"`
	SaleStatus                  string    `json:"saleStatus"`
	ExecutionAvailabilityStatus string    `json:"executionAvailabilityStatus"`
	InputSchemaSnapshot         string    `json:"inputSchemaSnapshot"`
}

type marketPublicInputSchema struct {
	SchemaVersion string                   `json:"schema_version"`
	Fields        []marketPublicInputField `json:"fields"`
	Instructions  []string                 `json:"instructions"`
	SampleRows    []map[string]any         `json:"sample_rows"`
}

func (s *marketPublicInputSchema) UnmarshalJSON(data []byte) error {
	type alias struct {
		SchemaVersion    string                   `json:"schema_version"`
		SchemaVersionAlt string                   `json:"schemaVersion"`
		Fields           []marketPublicInputField `json:"fields"`
		Instructions     []string                 `json:"instructions"`
		SampleRows       []map[string]any         `json:"sample_rows"`
		SampleRowsAlt    []map[string]any         `json:"sampleRows"`
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

type marketPublicInputField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	ValueType   string `json:"value_type"`
	SourceKind  string `json:"source_kind"`
}

func (f *marketPublicInputField) UnmarshalJSON(data []byte) error {
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

type marketInputPayload struct {
	ListingVersionID string           `json:"listingVersionId,omitempty"`
	ClientRequestID  string           `json:"clientRequestId,omitempty"`
	InputRows        []map[string]any `json:"inputRows"`
}

var forbiddenMarketInputFields = []string{
	"taskInputs",
	"task_inputs",
	"workflowDefinition",
	"workflow_definition",
	"templateSpec",
	"template_spec",
}

func newMarketListCmd(opts *rootOptions) *cobra.Command {
	var (
		keyword   string
		limit     int
		pageSize  int
		pageToken string
		orderBy   string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List published Market SkillBots",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("limit") && cmd.Flags().Changed("page-size") {
				return fmt.Errorf("--limit and --page-size cannot be used together")
			}
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			query := url.Values{}
			if strings.TrimSpace(keyword) != "" {
				query.Set("keyword", strings.TrimSpace(keyword))
			}
			if pageSize > 0 {
				query.Set("pageSize", fmt.Sprintf("%d", pageSize))
			} else if limit > 0 {
				query.Set("pageSize", fmt.Sprintf("%d", limit))
			}
			if strings.TrimSpace(pageToken) != "" {
				query.Set("pageToken", strings.TrimSpace(pageToken))
			}
			if strings.TrimSpace(orderBy) != "" {
				query.Set("orderBy", strings.TrimSpace(orderBy))
			}

			var raw map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/marketListings", query, &raw); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), raw)
			}
			resp, err := decodeJSONValue[marketListingsResponse](raw)
			if err != nil {
				return err
			}
			return printMarketListings(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&keyword, "keyword", "", "Search keyword matched against listing title and description")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of listings")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Page size")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "Opaque pagination token returned by the previous response")
	cmd.Flags().StringVar(&orderBy, "order-by", "", "Sort expression, for example 'createdAt desc'")
	return cmd
}

func newMarketShowCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show <listing-id>",
		Short: "Show one Market SkillBot detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			var raw map[string]any
			path := "/marketListings/" + url.PathEscape(strings.TrimSpace(args[0]))
			if err := httpClient.GetProductJSON(ctx, path, &raw); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), raw)
			}
			resp, err := decodeJSONValue[marketListingPublicResponse](raw)
			if err != nil {
				return err
			}
			return printMarketListingDetail(cmd.OutOrStdout(), resp)
		},
	}
}

func newMarketQuoteCmd(opts *rootOptions) *cobra.Command {
	var inputFile string
	cmd := &cobra.Command{
		Use:   "quote <listing-id>",
		Short: "Quote a Market SkillBot run from public input rows",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			listingID := strings.TrimSpace(args[0])
			input, err := prepareMarketInputPayload(ctx, cmd, httpClient, listingID, inputFile)
			if err != nil {
				return err
			}

			var resp map[string]any
			path := "/marketListings/" + url.PathEscape(listingID) + ":quote"
			if err := httpClient.PostProductJSON(ctx, path, marketQuotePayload(input), &resp); err != nil {
				return err
			}
			if err := maybeMapInsufficientBalanceError(opts, resp); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), resp)
			}
			return printMarketQuote(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&inputFile, "input-file", "", "JSON file with inputRows; prompts for one row when omitted")
	return cmd
}

func newMarketRunCmd(opts *rootOptions) *cobra.Command {
	var (
		inputFile       string
		clientRequestID string
		confirm         bool
	)
	cmd := &cobra.Command{
		Use:   "run <listing-id>",
		Short: "Quote and execute a Market SkillBot from public input rows",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			listingID := strings.TrimSpace(args[0])
			input, err := prepareMarketInputPayload(ctx, cmd, httpClient, listingID, inputFile)
			if err != nil {
				return err
			}

			var quoteResp map[string]any
			quotePath := "/marketListings/" + url.PathEscape(listingID) + ":quote"
			if err := httpClient.PostProductJSON(ctx, quotePath, marketQuotePayload(input), &quoteResp); err != nil {
				return err
			}
			if err := maybeMapInsufficientBalanceError(opts, quoteResp); err != nil {
				return err
			}
			if mapInsufficientBalance(quoteResp) {
				return errors.New("insufficient balance")
			}
			if !confirm {
				if opts.output == "json" {
					return writeIndentedJSON(cmd.OutOrStdout(), map[string]any{
						"quote":     quoteResp,
						"confirmed": false,
					})
				}
				if err := printMarketQuote(cmd.OutOrStdout(), quoteResp); err != nil {
					return err
				}
				_, err := fmt.Fprintln(cmd.ErrOrStderr(), "execution not submitted; pass --confirm to execute")
				return err
			}

			requestID, generatedRequestID, err := effectiveMarketClientRequestID(clientRequestID, input.ClientRequestID, listingID, input)
			if err != nil {
				return err
			}
			printGeneratedClientRequestID(cmd, requestID, generatedRequestID)

			var resp map[string]any
			executePath := "/marketListings/" + url.PathEscape(listingID) + ":execute"
			if err := httpClient.PostProductJSON(ctx, executePath, marketExecutePayload(input, requestID), &resp); err != nil {
				return err
			}
			opts.debugf(
				"market run: submitted listing_id=%s run_id=%s transaction_id=%s",
				listingID,
				stringMapValue(resp, "runId"),
				stringMapValue(resp, "runTransactionId"),
			)
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), resp)
			}
			return printMarketExecution(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&inputFile, "input-file", "", "JSON file with inputRows; prompts for one row when omitted")
	cmd.Flags().StringVar(&clientRequestID, "client-request-id", "", "Stable idempotency key for retrying the same confirmed execution")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm execution after the quote")
	return cmd
}

func newMarketWorkbookCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workbook",
		Short: "Run Market SkillBots with Excel workbooks",
	}
	cmd.AddCommand(
		newMarketWorkbookDownloadCmd(opts),
		newMarketWorkbookValidateCmd(opts),
		newMarketWorkbookQuoteCmd(opts),
		newMarketWorkbookRunCmd(opts),
	)
	return cmd
}

func newMarketWorkbookDownloadCmd(opts *rootOptions) *cobra.Command {
	var (
		outputPath       string
		listingVersionID string
	)
	cmd := &cobra.Command{
		Use:   "download <listing-id>",
		Short: "Download a Market SkillBot input workbook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			listingID := strings.TrimSpace(args[0])
			query := url.Values{}
			if strings.TrimSpace(listingVersionID) != "" {
				query.Set("listingVersionId", strings.TrimSpace(listingVersionID))
			}
			path := "/marketListings/" + url.PathEscape(listingID) + "/workbook"
			resp, err := httpClient.GetBinaryWithQuery(ctx, path, query)
			if err != nil {
				return err
			}

			filename := suggestedDownloadFilename(resp.ContentDisposition)
			if filename == "" {
				filename = listingID + ".xlsx"
			}
			targetPath, err := resolveFilePath(outputPath, filepath.Base(filename))
			if err != nil {
				return fmt.Errorf("resolve output file path: %w", err)
			}
			if err := os.WriteFile(targetPath, resp.Body, 0o644); err != nil {
				return fmt.Errorf("write market workbook: %w", err)
			}
			opts.debugf("market workbook: downloaded listing_id=%s filename=%s size_bytes=%d", listingID, filepath.Base(targetPath), len(resp.Body))

			result := map[string]any{
				"listingId":   listingID,
				"path":        targetPath,
				"filename":    filename,
				"size":        len(resp.Body),
				"contentType": resp.ContentType,
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), result)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "listing_id\t%s\npath\t%s\nsize\t%d\n", listingID, targetPath, len(resp.Body))
			return err
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output-file", "f", "", "Output .xlsx path or target directory")
	cmd.Flags().StringVar(&listingVersionID, "listing-version-id", "", "Optional Market listing version ID")
	return cmd
}

func newMarketWorkbookValidateCmd(opts *rootOptions) *cobra.Command {
	var workbookPath string
	cmd := &cobra.Command{
		Use:   "validate <listing-id>",
		Short: "Validate a filled Market SkillBot workbook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := postMarketWorkbook[validateTemplateFileResponse](cmd.Context(), opts, strings.TrimSpace(args[0]), workbookPath, ":validateWorkbook", nil)
			if err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), resp)
			}
			if err := printTemplateFileValidation(cmd.OutOrStdout(), resp); err != nil {
				return err
			}
			return templateFileValidationError(resp)
		},
	}
	cmd.Flags().StringVarP(&workbookPath, "file", "f", "", "Filled .xlsx workbook path")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newMarketWorkbookQuoteCmd(opts *rootOptions) *cobra.Command {
	var workbookPath string
	cmd := &cobra.Command{
		Use:   "quote <listing-id>",
		Short: "Quote a filled Market SkillBot workbook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := postMarketWorkbookMap(cmd.Context(), opts, strings.TrimSpace(args[0]), workbookPath, ":quoteWorkbook", nil)
			if err != nil {
				return err
			}
			if err := maybeMapInsufficientBalanceError(opts, resp); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), resp)
			}
			return printMarketQuote(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVarP(&workbookPath, "file", "f", "", "Filled .xlsx workbook path")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newMarketWorkbookRunCmd(opts *rootOptions) *cobra.Command {
	var (
		workbookPath    string
		clientRequestID string
		confirm         bool
	)
	cmd := &cobra.Command{
		Use:   "run <listing-id>",
		Short: "Quote and execute a Market SkillBot workbook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			listingID := strings.TrimSpace(args[0])
			quoteResp, err := postMarketWorkbookMap(cmd.Context(), opts, listingID, workbookPath, ":quoteWorkbook", nil)
			if err != nil {
				return err
			}
			if err := maybeMapInsufficientBalanceError(opts, quoteResp); err != nil {
				return err
			}
			if mapInsufficientBalance(quoteResp) {
				return errors.New("insufficient balance")
			}
			if !confirm {
				if opts.output == "json" {
					return writeIndentedJSON(cmd.OutOrStdout(), map[string]any{
						"quote":     quoteResp,
						"confirmed": false,
					})
				}
				if err := printMarketQuote(cmd.OutOrStdout(), quoteResp); err != nil {
					return err
				}
				_, err := fmt.Fprintln(cmd.ErrOrStderr(), "execution not submitted; pass --confirm to execute")
				return err
			}

			requestID, generatedRequestID, err := effectiveMarketWorkbookClientRequestID(clientRequestID, listingID, workbookPath)
			if err != nil {
				return err
			}
			printGeneratedClientRequestID(cmd, requestID, generatedRequestID)
			resp, err := postMarketWorkbookMap(cmd.Context(), opts, listingID, workbookPath, ":executeWorkbook", map[string]any{
				"confirm":         true,
				"clientRequestId": requestID,
			})
			if err != nil {
				return err
			}
			opts.debugf(
				"market workbook run: submitted listing_id=%s run_id=%s transaction_id=%s",
				listingID,
				stringMapValue(resp, "runId"),
				stringMapValue(resp, "runTransactionId"),
			)
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), resp)
			}
			return printMarketExecution(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVarP(&workbookPath, "file", "f", "", "Filled .xlsx workbook path")
	cmd.Flags().StringVar(&clientRequestID, "client-request-id", "", "Stable idempotency key for retrying the same confirmed workbook execution")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm execution after the quote")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func prepareMarketInputPayload(ctx context.Context, cmd *cobra.Command, httpClient *client.Client, listingID string, inputFile string) (marketInputPayload, error) {
	var listing marketListingPublicResponse
	path := "/marketListings/" + url.PathEscape(listingID)
	if err := httpClient.GetProductJSON(ctx, path, &listing); err != nil {
		return marketInputPayload{}, err
	}
	schema, err := parseMarketInputSchemaSnapshot(listing.InputSchemaSnapshot)
	if err != nil {
		return marketInputPayload{}, fmt.Errorf("parse inputSchemaSnapshot: %w", err)
	}
	if len(schema.Fields) == 0 {
		return marketInputPayload{}, errors.New("market listing inputSchemaSnapshot has no fields")
	}

	var input marketInputPayload
	if strings.TrimSpace(inputFile) == "" {
		rows, err := promptMarketInputRows(cmd.InOrStdin(), cmd.ErrOrStderr(), schema)
		if err != nil {
			return marketInputPayload{}, err
		}
		input.InputRows = rows
	} else {
		loaded, err := loadMarketInputPayload(inputFile)
		if err != nil {
			return marketInputPayload{}, err
		}
		input = loaded
	}
	if input.ListingVersionID != "" && listing.ListingVersionID != "" && input.ListingVersionID != listing.ListingVersionID {
		return marketInputPayload{}, fmt.Errorf(
			"listingVersionId %q does not match current listing version %q; inputRows are validated against the current public input schema",
			input.ListingVersionID,
			listing.ListingVersionID,
		)
	}
	if input.ListingVersionID == "" {
		input.ListingVersionID = listing.ListingVersionID
	}
	rows, err := validateMarketInputRows(input.InputRows, schema)
	if err != nil {
		return marketInputPayload{}, err
	}
	input.InputRows = rows
	return input, nil
}

func decodeJSONValue[T any](value any) (T, error) {
	var out T
	data, err := json.Marshal(value)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func parseMarketInputSchemaSnapshot(snapshot string) (marketPublicInputSchema, error) {
	snapshot = strings.TrimSpace(snapshot)
	if snapshot == "" {
		return marketPublicInputSchema{}, errors.New("inputSchemaSnapshot is empty")
	}
	var schema marketPublicInputSchema
	if err := json.Unmarshal([]byte(snapshot), &schema); err != nil {
		return marketPublicInputSchema{}, err
	}
	return schema, nil
}

func loadMarketInputPayload(filePath string) (marketInputPayload, error) {
	data, err := os.ReadFile(strings.TrimSpace(filePath))
	if err != nil {
		return marketInputPayload{}, fmt.Errorf("read input file: %w", err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return marketInputPayload{}, errors.New("input file is empty")
	}
	if trimmed[0] == '[' {
		var rows []map[string]any
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return marketInputPayload{}, fmt.Errorf("parse inputRows array: %w", err)
		}
		return marketInputPayload{InputRows: rows}, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return marketInputPayload{}, fmt.Errorf("parse input JSON: %w", err)
	}
	for _, field := range forbiddenMarketInputFields {
		if value, ok := raw[field]; ok && len(bytes.TrimSpace(value)) > 0 {
			return marketInputPayload{}, fmt.Errorf("%s is an internal Market execution field; use inputRows instead", field)
		}
	}
	if _, ok := raw["inputRows"]; !ok {
		return marketInputPayload{}, errors.New("input JSON must contain inputRows or be a JSON array of rows")
	}

	var payload marketInputPayload
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		return marketInputPayload{}, fmt.Errorf("parse inputRows payload: %w", err)
	}
	return payload, nil
}

func validateMarketInputRows(rows []map[string]any, schema marketPublicInputSchema) ([]map[string]any, error) {
	if len(rows) == 0 {
		return nil, errors.New("inputRows is required")
	}
	fieldsByKey := make(map[string]marketPublicInputField, len(schema.Fields))
	requiredKeys := make([]string, 0)
	for _, field := range schema.Fields {
		key := strings.TrimSpace(field.Key)
		if key == "" {
			continue
		}
		fieldsByKey[key] = field
		if field.Required {
			requiredKeys = append(requiredKeys, key)
		}
	}
	if len(fieldsByKey) == 0 {
		return nil, errors.New("input schema has no keyed fields")
	}

	out := make([]map[string]any, 0, len(rows))
	for idx, row := range rows {
		values := make(map[string]any, len(row))
		for key, value := range row {
			field, ok := fieldsByKey[key]
			if !ok {
				return nil, fmt.Errorf("inputRows[%d] field %q is not declared by inputSchemaSnapshot", idx, key)
			}
			if value == nil {
				return nil, fmt.Errorf("inputRows[%d] field %q: null is not supported", idx, key)
			}
			if err := validateMarketInputValue(field, value); err != nil {
				return nil, fmt.Errorf("inputRows[%d] field %q: %w", idx, key, err)
			}
			values[key] = value
		}
		for _, key := range requiredKeys {
			value, ok := values[key]
			if !ok || marketInputValueEmpty(value) {
				return nil, fmt.Errorf("inputRows[%d] field %q is required", idx, key)
			}
		}
		out = append(out, values)
	}
	return out, nil
}

func validateMarketInputValue(field marketPublicInputField, value any) error {
	valueType := strings.ToLower(strings.TrimSpace(field.ValueType))
	switch valueType {
	case "", "string", "text":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("must be a string")
		}
	case "number", "float", "double":
		if !isJSONNumber(value) {
			return fmt.Errorf("must be a number")
		}
	case "integer", "int":
		if !isJSONInteger(value) {
			return fmt.Errorf("must be an integer")
		}
	case "boolean", "bool":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("must be a boolean")
		}
	default:
		if _, err := scalarToString(value); err != nil {
			return fmt.Errorf("must be a scalar value")
		}
	}
	return nil
}

func isJSONNumber(value any) bool {
	switch value.(type) {
	case float64, float32, int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
		return true
	default:
		return false
	}
}

func isJSONInteger(value any) bool {
	switch v := value.(type) {
	case float64:
		return math.Trunc(v) == v
	case float32:
		return math.Trunc(float64(v)) == float64(v)
	case int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
		return true
	default:
		return false
	}
}

func marketInputValueEmpty(value any) bool {
	if value == nil {
		return true
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}

func promptMarketInputRows(r io.Reader, w io.Writer, schema marketPublicInputSchema) ([]map[string]any, error) {
	reader := bufio.NewReader(r)
	row := make(map[string]any, len(schema.Fields))
	for _, field := range schema.Fields {
		label := strings.TrimSpace(field.Label)
		if label == "" {
			label = field.Key
		}
		required := ""
		if field.Required {
			required = " required"
		}
		if _, err := fmt.Fprintf(w, "%s (%s%s): ", label, field.Key, required); err != nil {
			return nil, err
		}
		raw, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		value := strings.TrimSpace(raw)
		if value == "" {
			if field.Required {
				return nil, fmt.Errorf("field %q is required", field.Key)
			}
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}
		parsed, err := parseMarketInputString(field, value)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", field.Key, err)
		}
		row[field.Key] = parsed
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return []map[string]any{row}, nil
}

func parseMarketInputString(field marketPublicInputField, value string) (any, error) {
	switch strings.ToLower(strings.TrimSpace(field.ValueType)) {
	case "number", "float", "double":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("parse number: %w", err)
		}
		return parsed, nil
	case "integer", "int":
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse integer: %w", err)
		}
		return parsed, nil
	case "boolean", "bool":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("parse boolean: %w", err)
		}
		return parsed, nil
	default:
		return value, nil
	}
}

func effectiveMarketClientRequestID(flagValue string, fileValue string, listingID string, input marketInputPayload) (string, bool, error) {
	flagValue = strings.TrimSpace(flagValue)
	if flagValue != "" {
		return flagValue, false, nil
	}
	fileValue = strings.TrimSpace(fileValue)
	if fileValue != "" {
		return fileValue, false, nil
	}
	value, err := stableMarketJSONClientRequestID(listingID, input)
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func effectiveMarketWorkbookClientRequestID(flagValue string, listingID string, workbookPath string) (string, bool, error) {
	flagValue = strings.TrimSpace(flagValue)
	if flagValue != "" {
		return flagValue, false, nil
	}
	content, err := os.ReadFile(strings.TrimSpace(workbookPath))
	if err != nil {
		return "", false, fmt.Errorf("read workbook for clientRequestId: %w", err)
	}
	value, err := stableMarketClientRequestID(map[string]any{
		"mode":      "workbook",
		"listingId": strings.TrimSpace(listingID),
		"filename":  filepath.Base(workbookPath),
		"sha256":    fmt.Sprintf("%x", sha256.Sum256(content)),
	})
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func stableMarketJSONClientRequestID(listingID string, input marketInputPayload) (string, error) {
	return stableMarketClientRequestID(map[string]any{
		"mode":      "json",
		"listingId": strings.TrimSpace(listingID),
		"inputRows": input.InputRows,
	})
}

func stableMarketClientRequestID(seed any) (string, error) {
	data, err := json.Marshal(seed)
	if err != nil {
		return "", fmt.Errorf("build stable clientRequestId: %w", err)
	}
	sum := sha256.Sum256(data)
	return "loomloom-cli-market-" + fmt.Sprintf("%x", sum[:16]), nil
}

func marketQuotePayload(input marketInputPayload) map[string]any {
	return map[string]any{
		"inputRows": input.InputRows,
	}
}

func marketExecutePayload(input marketInputPayload, clientRequestID string) map[string]any {
	payload := marketQuotePayload(input)
	payload["confirm"] = true
	payload["clientRequestId"] = clientRequestID
	return payload
}

func postMarketWorkbookMap(ctx context.Context, opts *rootOptions, listingID string, workbookPath string, action string, extra map[string]any) (map[string]any, error) {
	return postMarketWorkbook[map[string]any](ctx, opts, listingID, workbookPath, action, extra)
}

func postMarketWorkbook[T any](ctx context.Context, opts *rootOptions, listingID string, workbookPath string, action string, extra map[string]any) (T, error) {
	var out T
	payload, err := workbookPayload(workbookPath, nil)
	if err != nil {
		return out, err
	}
	for key, value := range extra {
		payload[key] = value
	}
	httpClient, err := newHTTPClient(opts)
	if err != nil {
		return out, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	endpoint := "/marketListings/" + url.PathEscape(listingID) + action
	if err := httpClient.PostProductJSON(requestCtx, endpoint, payload, &out); err != nil {
		return out, err
	}
	return out, nil
}

func printMarketListings(w io.Writer, resp marketListingsResponse) error {
	if len(resp.Items) == 0 {
		if _, err := fmt.Fprintln(w, "no market listings"); err != nil {
			return err
		}
		if resp.NextPageToken != "" {
			_, err := fmt.Fprintf(w, "next_page_token\t%s\n", resp.NextPageToken)
			return err
		}
		return nil
	}
	tw := newTabWriter(w)
	if _, err := fmt.Fprintln(tw, "id\tname\ttask_fixed_fee\tavailability\tversion\tdescription"); err != nil {
		return err
	}
	for _, item := range resp.Items {
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			oneLine(item.DisplayName),
			formatMoneyT(int64(item.TaskFixedFeeT), item.Currency),
			oneLine(item.ExecutionAvailabilityStatus),
			oneLine(item.ListingVersionID),
			oneLine(item.Description),
		); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if resp.NextPageToken != "" {
		_, err := fmt.Fprintf(w, "next_page_token\t%s\n", resp.NextPageToken)
		return err
	}
	return nil
}

func printMarketListingDetail(w io.Writer, listing marketListingPublicResponse) error {
	tw := newTabWriter(w)
	for _, row := range [][2]string{
		{"id", listing.ID},
		{"display_name", listing.DisplayName},
		{"description", listing.Description},
		{"listing_version_id", listing.ListingVersionID},
		{"task_fixed_fee", formatMoneyT(int64(listing.TaskFixedFeeT), listing.Currency)},
		{"sale_status", listing.SaleStatus},
		{"execution_availability_status", listing.ExecutionAvailabilityStatus},
	} {
		if row[1] == "" {
			continue
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", row[0], oneLine(row[1])); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	schema, err := parseMarketInputSchemaSnapshot(listing.InputSchemaSnapshot)
	if err != nil {
		_, err = fmt.Fprintf(w, "input_schema_error\t%s\n", err)
		return err
	}
	if schema.SchemaVersion != "" {
		if _, err := fmt.Fprintf(w, "input_schema_version\t%s\n", schema.SchemaVersion); err != nil {
			return err
		}
	}
	if len(schema.Instructions) > 0 {
		if _, err := fmt.Fprintln(w, "instructions:"); err != nil {
			return err
		}
		for _, instruction := range schema.Instructions {
			if _, err := fmt.Fprintf(w, "- %s\n", instruction); err != nil {
				return err
			}
		}
	}
	if len(schema.Fields) > 0 {
		if _, err := fmt.Fprintln(w, "fields:"); err != nil {
			return err
		}
		fieldWriter := newTabWriter(w)
		if _, err := fmt.Fprintln(fieldWriter, "label\tkey\trequired\ttype\tsource\tdescription"); err != nil {
			return err
		}
		for _, field := range schema.Fields {
			if _, err := fmt.Fprintf(
				fieldWriter,
				"%s\t%s\t%t\t%s\t%s\t%s\n",
				oneLine(field.Label),
				field.Key,
				field.Required,
				field.ValueType,
				field.SourceKind,
				oneLine(field.Description),
			); err != nil {
				return err
			}
		}
		if err := fieldWriter.Flush(); err != nil {
			return err
		}
	}
	if len(schema.SampleRows) > 0 {
		if _, err := fmt.Fprintln(w, "sample_rows:"); err != nil {
			return err
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(schema.SampleRows)
	}
	return nil
}

func printMarketQuote(w io.Writer, resp map[string]any) error {
	currency := stringMapValue(resp, "currency")
	if currency == "-" {
		currency = ""
	}
	tw := newTabWriter(w)
	for _, key := range []string{"quoteId", "listingVersionId", "currency"} {
		if err := printStringMapField(tw, resp, key); err != nil {
			return err
		}
	}
	if err := printMoneyMapField(tw, resp, "estimated_execution_cost", "estimatedExecutionCostT", currency); err != nil {
		return err
	}
	if err := printMoneyMapField(tw, resp, "task_fixed_fee", "taskFixedFeeT", currency); err != nil {
		return err
	}
	if value, ok := resp["taskCount"]; ok && value != nil {
		if _, err := fmt.Fprintf(tw, "taskCount\t%s\n", displayJSONValue(value)); err != nil {
			return err
		}
	}
	if err := printMoneyMapField(tw, resp, "estimated_payable", "estimatedBuyerPayableT", currency); err != nil {
		return err
	}
	return tw.Flush()
}

func printMarketExecution(w io.Writer, resp map[string]any) error {
	currency := stringMapValue(resp, "currency")
	if currency == "-" {
		currency = ""
	}
	tw := newTabWriter(w)
	for _, key := range []string{"runTransactionId", "runId", "listingId", "listingVersionId", "skillName", "currency"} {
		if err := printStringMapField(tw, resp, key); err != nil {
			return err
		}
	}
	if err := printMoneyMapField(tw, resp, "task_fixed_fee", "taskFixedFeeT", currency); err != nil {
		return err
	}
	if err := printMoneyMapField(tw, resp, "estimated_execution_cost", "estimatedExecutionCostT", currency); err != nil {
		return err
	}
	if err := printMoneyMapField(tw, resp, "estimated_payable", "estimatedBuyerPayableT", currency); err != nil {
		return err
	}
	if err := printMoneyMapField(tw, resp, "actual_execution_cost", "actualExecutionCostT", currency); err != nil {
		return err
	}
	if err := printMoneyMapField(tw, resp, "final_payable", "finalBuyerPayableT", currency); err != nil {
		return err
	}
	if err := printStringMapField(tw, resp, "transactionStatus"); err != nil {
		return err
	}
	return tw.Flush()
}

// printStringMapField writes a "key<TAB>value" row for a string field in a
// decoded JSON map. It is a no-op when the field is absent, nil, or an empty
// string so the backend omitting a value does not leave a blank row.
func printStringMapField(tw *tabwriter.Writer, resp map[string]any, key string) error {
	value, ok := resp[key]
	if !ok || value == nil {
		return nil
	}
	rendered := displayJSONValue(value)
	if strings.TrimSpace(rendered) == "" {
		return nil
	}
	_, err := fmt.Fprintf(tw, "%s\t%s\n", key, rendered)
	return err
}

// printMoneyMapField writes a readable-money row for a raw *T field present in
// a decoded JSON map. It is a no-op if the field is absent.
func printMoneyMapField(tw *tabwriter.Writer, resp map[string]any, label string, rawKey string, currency string) error {
	amountT, ok := int64MapValue(resp, rawKey)
	if !ok {
		return nil
	}
	_, err := fmt.Fprintf(tw, "%s\t%s\n", label, formatMoneyT(amountT, currency))
	return err
}

func displayJSONValue(value any) string {
	switch v := value.(type) {
	case float64:
		if math.Trunc(v) == v {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return oneLine(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func oneLine(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\n", " ")
	value = strings.ReplaceAll(value, "\t", " ")
	if len(value) > 160 {
		return value[:157] + "..."
	}
	return value
}

func newDeprecatedMarketPublishCmd(opts *rootOptions) *cobra.Command {
	var (
		listingID         string
		templateID        string
		templateVersionID string
		displayName       string
		description       string
		taskFixedFee      string
		taskFixedFeeT     int64
	)
	cmd := &cobra.Command{
		Use:        "publish",
		Short:      "Publish a template version as a Market SkillBot",
		Deprecated: "use 'loomloom listing publish <template-id>'",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedTaskFixedFeeT, err := taskFixedFeeFromFlags(cmd, taskFixedFee, taskFixedFeeT)
			if err != nil {
				return err
			}

			req := publishMarketListingRequest{
				ListingID:         strings.TrimSpace(listingID),
				DisplayName:       strings.TrimSpace(displayName),
				Description:       strings.TrimSpace(description),
				TaskFixedFeeT:     resolvedTaskFixedFeeT,
				TemplateID:        strings.TrimSpace(templateID),
				TemplateVersionID: strings.TrimSpace(templateVersionID),
			}

			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			var resp map[string]any
			if err := httpClient.PostProductJSON(ctx, "/marketListings", req, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&listingID, "listing-id", "", "Existing listing ID when publishing a new version")
	cmd.Flags().StringVar(&templateID, "template-id", "", "Template ID to publish")
	cmd.Flags().StringVar(&templateVersionID, "template-version-id", "", "Template version ID to publish")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Market SkillBot display name")
	cmd.Flags().StringVar(&description, "description", "", "Market SkillBot description")
	cmd.Flags().StringVar(&taskFixedFee, "task-fixed-fee", "", "Creator fixed fee per billable task, in currency units (for example 0.5)")
	cmd.Flags().Int64Var(&taskFixedFeeT, "task-fixed-fee-t", 0, "Deprecated: creator fixed fee per billable task, in raw API units")
	_ = cmd.Flags().MarkDeprecated("task-fixed-fee-t", "use --task-fixed-fee with a decimal currency amount")
	_ = cmd.MarkFlagRequired("template-id")
	_ = cmd.MarkFlagRequired("template-version-id")
	_ = cmd.MarkFlagRequired("display-name")
	return cmd
}

func newDeprecatedMarketRelistCmd(opts *rootOptions) *cobra.Command {
	cmd := newMarketSaleStatusCmd(opts, "relist", "Restore a Market SkillBot listing", "list")
	cmd.Deprecated = "use 'loomloom listing relist <listing-id>'"
	return cmd
}

func newMarketSaleStatusCmd(opts *rootOptions, use string, short string, action string) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <listing-id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			query := url.Values{}
			var resp map[string]any
			path := "/marketListings/" + url.PathEscape(strings.TrimSpace(args[0])) + ":" + action
			if err := httpClient.PostProductJSONWithQuery(ctx, path, query, nil, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
}
