package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cogfoundry-labs/loomloom/cli/internal/client"
	"github.com/spf13/cobra"
)

func newRunCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Submit and monitor template runs",
	}

	cmd.AddCommand(
		newRunValidateCmd(opts),
		newRunPrecheckCmd(opts),
		newRunExecuteCmd(opts),
		newRunSubmitCmd(opts),
		newRunWatchCmd(opts),
		newRunListCmd(opts),
		newRunGetCmd(opts),
		newRunResultRowsCmd(opts),
		newRunResultWorkbookCmd(opts),
	)
	return cmd
}

func newRunValidateCmd(opts *rootOptions) *cobra.Command {
	var inputPath string

	cmd := &cobra.Command{
		Use:   "validate <template-id>",
		Short: "Validate official template rows from JSON or JSONL without submitting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := strings.TrimSpace(args[0])
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			rows, err := prepareOfficialTemplateRows(ctx, httpClient, templateID, inputPath)
			if err != nil {
				return err
			}
			var resp validateTemplateRowsResponse
			if err := httpClient.PostJSON(
				ctx,
				"/officialTemplates/"+templateID+":validateRows",
				map[string]any{"rows": templateRowsPayload(rows)},
				&resp,
			); err != nil {
				return err
			}

			if opts.output == "json" {
				if err := writeIndentedJSON(cmd.OutOrStdout(), map[string]any{
					"templateId": templateID,
					"inputPath":  strings.TrimSpace(inputPath),
					"rowCount":   len(rows),
					"validation": resp,
				}); err != nil {
					return err
				}
			} else if resp.Valid {
				if _, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"template_id\t%s\ninput_path\t%s\nrow_count\t%d\nvalid\ttrue\n",
					templateID,
					strings.TrimSpace(inputPath),
					len(rows),
				); err != nil {
					return err
				}
			}
			return validationError(resp)
		},
	}
	cmd.Flags().StringVarP(&inputPath, "file", "f", "", "Input file in JSON array or JSONL format")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newRunPrecheckCmd(opts *rootOptions) *cobra.Command {
	var inputPath string

	cmd := &cobra.Command{
		Use:   "precheck <template-id>",
		Short: "Estimate official template row cost without submitting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := strings.TrimSpace(args[0])
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			rows, err := prepareOfficialTemplateRows(ctx, httpClient, templateID, inputPath)
			if err != nil {
				return err
			}
			var resp precheckTemplateRowsResponse
			if err := httpClient.PostJSON(
				ctx,
				"/officialTemplates/"+templateID+":precheckRows",
				map[string]any{"rows": templateRowsPayload(rows)},
				&resp,
			); err != nil {
				return err
			}
			if balance := resp.BalanceCheck; balance != nil && !balance.IsSufficient {
				if err := maybeInsufficientBalanceError(opts, balance); err != nil {
					return err
				}
				estimatedCost, err := formatResponseMoney(resp.EstimatedTotalCost, resp.EstimatedTotalCostT, balance.Currency)
				if err != nil {
					return err
				}
				availableBalance, err := formatResponseMoney(
					balance.AvailableBalanceMoney,
					balance.AvailableBalance,
					balance.Currency,
				)
				if err != nil {
					return err
				}
				return fmt.Errorf(
					"insufficient balance: estimated_cost=%s available=%s",
					estimatedCost,
					availableBalance,
				)
			}

			if opts.output == "json" {
				result := precheckJSONPayload(resp)
				result["templateId"] = templateID
				result["inputPath"] = strings.TrimSpace(inputPath)
				result["rowCount"] = len(rows)
				return writeIndentedJSON(cmd.OutOrStdout(), result)
			}
			if _, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"template_id\t%s\ninput_path\t%s\nrow_count\t%d\n",
				templateID,
				strings.TrimSpace(inputPath),
				len(rows),
			); err != nil {
				return err
			}
			return printPrecheck(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVarP(&inputPath, "file", "f", "", "Input file in JSON array or JSONL format")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newRunExecuteCmd(opts *rootOptions) *cobra.Command {
	var (
		inputPath       string
		callbackURL     string
		clientRequestID string
	)

	cmd := &cobra.Command{
		Use:   "execute <template-id>",
		Short: "Submit prechecked official template rows from JSON or JSONL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := strings.TrimSpace(args[0])
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			rows, err := prepareOfficialTemplateRows(ctx, httpClient, templateID, inputPath)
			if err != nil {
				return err
			}
			requestID, generatedRequestID := effectiveClientRequestID(clientRequestID)
			payload := map[string]any{
				"rows":            templateRowsPayload(rows),
				"clientRequestId": requestID,
			}
			if callbackURL != "" {
				payload["callbackUrl"] = callbackURL
			}

			printGeneratedClientRequestID(cmd, requestID, generatedRequestID)
			var resp submitTemplateRowsResponse
			if err := httpClient.PostJSON(
				ctx,
				"/officialTemplates/"+templateID+":runRows",
				payload,
				&resp,
			); err != nil {
				return err
			}
			opts.debugf(
				"official template run: submitted template_id=%s run_id=%s row_count=%d",
				templateID,
				resp.RunID,
				len(rows),
			)

			result := map[string]any{
				"templateId":      templateID,
				"inputPath":       strings.TrimSpace(inputPath),
				"rowCount":        len(rows),
				"clientRequestId": requestID,
				"runId":           resp.RunID,
				"status":          resp.Status,
				"acceptedAt":      int64(resp.AcceptedAt),
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), result)
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"template_id\t%s\ninput_path\t%s\nrow_count\t%d\nrun_id\t%s\nstatus\t%s\naccepted_at\t%s\n",
				templateID,
				strings.TrimSpace(inputPath),
				len(rows),
				resp.RunID,
				resp.Status,
				formatUnix(int64(resp.AcceptedAt)),
			)
			return err
		},
	}
	cmd.Flags().StringVarP(&inputPath, "file", "f", "", "Input file in JSON array or JSONL format")
	cmd.Flags().StringVar(&callbackURL, "callback-url", "", "Optional callback URL")
	cmd.Flags().StringVar(&clientRequestID, "client-request-id", "", "Stable idempotency key for retrying the same rows submission")
	cmd.Flags().StringVar(&clientRequestID, "idempotency-key", "", "Deprecated alias for --client-request-id")
	_ = cmd.Flags().MarkDeprecated("idempotency-key", "use --client-request-id")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func prepareOfficialTemplateRows(
	ctx context.Context,
	httpClient *client.Client,
	templateID string,
	inputPath string,
) ([]templateDisplayRow, error) {
	inputPath = strings.TrimSpace(inputPath)
	if inputPath == "" {
		return nil, fmt.Errorf("--file is required")
	}
	rows, err := loadTemplateRows(inputPath)
	if err != nil {
		return nil, err
	}
	var schemaResp templateSchemaResponse
	if err := httpClient.GetJSON(ctx, "/officialTemplates/"+templateID+"/schema", &schemaResp); err != nil {
		return nil, err
	}
	return remapRowsToHeaderLabels(rows, schemaResp), nil
}

func newRunListCmd(opts *rootOptions) *cobra.Command {
	var (
		status    string
		pageSize  int
		pageToken string
		orderBy   string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Product API runs with optional Market context",
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
			if strings.TrimSpace(pageToken) != "" {
				query.Set("pageToken", strings.TrimSpace(pageToken))
			}
			if strings.TrimSpace(orderBy) != "" {
				normalizedOrderBy, err := normalizeRunOrderBy(orderBy)
				if err != nil {
					return err
				}
				query.Set("orderBy", normalizedOrderBy)
			}

			var resp map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/users/me/runs", query, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Run status filter")
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "Page size")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "Page token")
	cmd.Flags().StringVar(&orderBy, "order-by", "", "Order by createdAt or updatedAt (asc or desc)")
	return cmd
}

func normalizeRunOrderBy(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "createdat", "created_at")
	normalized = strings.ReplaceAll(normalized, "updatedat", "updated_at")
	normalized = strings.Join(strings.Fields(normalized), "_")

	switch normalized {
	case "created_at_asc", "created_at_desc", "updated_at_asc", "updated_at_desc":
		return normalized, nil
	default:
		return "", fmt.Errorf(
			"invalid --order-by %q: use createdAt asc, createdAt desc, updatedAt asc, or updatedAt desc",
			value,
		)
	}
}

func newRunGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <run-id>",
		Short: "Get one Product API run detail with optional Market context",
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
			path := "/users/me/runs/" + url.PathEscape(strings.TrimSpace(args[0]))
			if err := httpClient.GetProductJSONWithQuery(ctx, path, query, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
}

func newRunSubmitCmd(opts *rootOptions) *cobra.Command {
	var (
		inputPath       string
		callbackURL     string
		clientRequestID string
	)

	cmd := &cobra.Command{
		Use:    "submit <template-id>",
		Short:  "Legacy combined validate, precheck, and submit flow for official template rows",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputPath == "" {
				return fmt.Errorf("--file is required")
			}

			rows, err := loadTemplateRows(inputPath)
			if err != nil {
				return err
			}

			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			var schemaResp templateSchemaResponse
			if err := httpClient.GetJSON(ctx, "/officialTemplates/"+args[0]+"/schema", &schemaResp); err != nil {
				return err
			}
			rows = remapRowsToHeaderLabels(rows, schemaResp)

			requestID, generatedRequestID := effectiveClientRequestID(clientRequestID)
			payload := map[string]any{
				"rows":            templateRowsPayload(rows),
				"clientRequestId": requestID,
			}
			if callbackURL != "" {
				payload["callbackUrl"] = callbackURL
			}

			var validateResp validateTemplateRowsResponse
			if err := httpClient.PostJSON(ctx, "/officialTemplates/"+args[0]+":validateRows", payload, &validateResp); err != nil {
				return err
			}
			if !validateResp.Valid {
				if opts.output == "json" {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					_ = enc.Encode(map[string]any{
						"templateId": args[0],
						"inputPath":  inputPath,
						"validation": validateResp,
					})
				}
				return validationError(validateResp)
			}

			var precheckResp precheckTemplateRowsResponse
			if err := httpClient.PostJSON(ctx, "/officialTemplates/"+args[0]+":precheckRows", payload, &precheckResp); err != nil {
				return err
			}
			if balance := precheckResp.BalanceCheck; balance != nil && !balance.IsSufficient {
				if err := maybeInsufficientBalanceError(opts, balance); err != nil {
					return err
				}
				estimatedCost, err := formatResponseMoney(
					precheckResp.EstimatedTotalCost,
					precheckResp.EstimatedTotalCostT,
					balance.Currency,
				)
				if err != nil {
					return err
				}
				availableBalance, err := formatResponseMoney(
					balance.AvailableBalanceMoney,
					balance.AvailableBalance,
					balance.Currency,
				)
				if err != nil {
					return err
				}
				return fmt.Errorf(
					"insufficient balance: estimated_cost=%s available=%s",
					estimatedCost,
					availableBalance,
				)
			}

			printGeneratedClientRequestID(cmd, requestID, generatedRequestID)
			var submitResp submitTemplateRowsResponse
			if err := httpClient.PostJSON(ctx, "/officialTemplates/"+args[0]+":runRows", payload, &submitResp); err != nil {
				return err
			}
			opts.debugf("official template run: submitted template_id=%s run_id=%s row_count=%d", args[0], submitResp.RunID, len(rows))

			result := map[string]any{
				"templateId":      args[0],
				"inputPath":       inputPath,
				"rowCount":        len(rows),
				"balanceCheck":    precheckResp.BalanceCheck,
				"clientRequestId": requestID,
				"runId":           submitResp.RunID,
				"status":          submitResp.Status,
				"acceptedAt":      int64(submitResp.AcceptedAt),
			}
			if precheckResp.EstimatedTotalCostT != nil {
				result["estimatedTotalCostT"] = int64(*precheckResp.EstimatedTotalCostT)
			}
			if precheckResp.EstimatedTotalCost != nil {
				result["estimatedTotalCost"] = precheckResp.EstimatedTotalCost
			}

			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			if err := printPrecheck(cmd.OutOrStdout(), precheckResp); err != nil {
				return err
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"template_id\t%s\ninput_path\t%s\nrow_count\t%d\nrun_id\t%s\nstatus\t%s\naccepted_at\t%s\n",
				args[0],
				inputPath,
				len(rows),
				submitResp.RunID,
				submitResp.Status,
				formatUnix(int64(submitResp.AcceptedAt)),
			)
			return err
		},
	}
	cmd.Flags().StringVarP(&inputPath, "file", "f", "", "Input file in JSON array or JSONL format")
	cmd.Flags().StringVar(&callbackURL, "callback-url", "", "Optional callback URL")
	cmd.Flags().StringVar(&clientRequestID, "client-request-id", "", "Stable idempotency key for retrying the same rows submission")
	cmd.Flags().StringVar(&clientRequestID, "idempotency-key", "", "Deprecated alias for --client-request-id")
	_ = cmd.Flags().MarkDeprecated("idempotency-key", "use --client-request-id")
	return cmd
}

func newRunWatchCmd(opts *rootOptions) *cobra.Command {
	interval := 5 * time.Second
	var maxWait time.Duration

	cmd := &cobra.Command{
		Use:   "watch <run-id>",
		Short: "Poll a run until it reaches a terminal state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}

			watchCtx := cmd.Context()
			if maxWait > 0 {
				var cancel context.CancelFunc
				watchCtx, cancel = context.WithTimeout(cmd.Context(), maxWait)
				defer cancel()
			}

			var wrap runGetResponse
			for {
				ctx, cancel := context.WithTimeout(watchCtx, opts.timeout)
				err := httpClient.GetJSON(ctx, "/users/me/runs/"+args[0], &wrap)
				cancel()
				if err != nil {
					return err
				}
				latest := wrap.Run

				if opts.output != "json" {
					actualCost, err := formatResponseMoney(latest.ActualCost, latest.ActualCostT, "")
					if err != nil {
						return err
					}
					_, err = fmt.Fprintf(
						cmd.OutOrStdout(),
						"status=%s completed=%d/%d failed=%d cancelled=%d cost=%s\n",
						latest.Status,
						int(latest.CompletedTasks),
						int(latest.TotalTasks),
						int(latest.FailedTasks),
						int(latest.CancelledTasks),
						actualCost,
					)
					if err != nil {
						return err
					}
				}

				if isTerminalRunStatus(latest.Status) {
					if opts.output == "json" {
						enc := json.NewEncoder(cmd.OutOrStdout())
						enc.SetIndent("", "  ")
						return enc.Encode(latest)
					}
					return printRunSummary(cmd.OutOrStdout(), latest)
				}

				select {
				case <-watchCtx.Done():
					if maxWait <= 0 {
						return watchCtx.Err()
					}
					return fmt.Errorf("timed out after %s waiting for run %s (current status: %s)", maxWait, args[0], latest.Status)
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", interval, "Polling interval")
	cmd.Flags().DurationVar(&maxWait, "max-wait", 0, "Maximum total time to wait for the run to finish; 0 means wait forever")
	return cmd
}

type runResultRowArtifact struct {
	ArtifactID string `json:"artifactId"`
	TaskID     string `json:"taskId"`
	StepID     string `json:"stepId"`
	PortName   string `json:"portName"`
	MimeType   string `json:"mimeType"`
	AccessURL  string `json:"accessUrl"`
	InlineText string `json:"inlineText"`
}

type runResultRow struct {
	RowIndex     int                    `json:"rowIndex"`
	Status       string                 `json:"status"`
	Error        string                 `json:"error"`
	ErrorMessage string                 `json:"errorMessage"`
	InputJSON    string                 `json:"inputJson"`
	Artifacts    []runResultRowArtifact `json:"artifacts"`
}

type listRunResultRowsResponse struct {
	Rows          []runResultRow `json:"rows"`
	NextPageToken string         `json:"nextPageToken"`
	TotalCount    int            `json:"totalCount"`
}

func (r *listRunResultRowsResponse) UnmarshalJSON(data []byte) error {
	type alias struct {
		Rows          []runResultRow `json:"rows"`
		Items         []runResultRow `json:"items"`
		NextPageToken string         `json:"nextPageToken"`
		TotalCount    int            `json:"totalCount"`
	}
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	r.Rows = parsed.Rows
	if len(r.Rows) == 0 {
		r.Rows = parsed.Items
	}
	for i := range r.Rows {
		if r.Rows[i].Error == "" {
			r.Rows[i].Error = r.Rows[i].ErrorMessage
		}
	}
	r.NextPageToken = parsed.NextPageToken
	r.TotalCount = parsed.TotalCount
	return nil
}

func newRunResultRowsCmd(opts *rootOptions) *cobra.Command {
	var (
		pageSize  int
		pageToken string
	)

	cmd := &cobra.Command{
		Use:   "result-rows <run-id>",
		Short: "List run results joined with the persisted input snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			query := url.Values{}
			if pageSize > 0 {
				query.Set("pageSize", fmt.Sprintf("%d", pageSize))
			}
			if strings.TrimSpace(pageToken) != "" {
				query.Set("pageToken", strings.TrimSpace(pageToken))
			}

			var resp listRunResultRowsResponse
			path := "/users/me/runs/" + strings.TrimSpace(args[0]) + "/resultRows"
			if err := httpClient.GetJSONWithQuery(ctx, path, query, &resp); err != nil {
				return err
			}

			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			if _, err := fmt.Fprintln(tw, "row\tstatus\tartifacts\tresult\tinput"); err != nil {
				return err
			}
			for _, row := range resp.Rows {
				input := strings.ReplaceAll(strings.TrimSpace(row.InputJSON), "\n", " ")
				if len(input) > 120 {
					input = input[:117] + "..."
				}
				if _, err := fmt.Fprintf(tw, "%d\t%s\t%d\t%s\t%s\n", row.RowIndex, row.Status, len(row.Artifacts), runResultPreview(row), input); err != nil {
					return err
				}
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "total_count\t%d\nnext_page_token\t%s\n", resp.TotalCount, resp.NextPageToken)
			return err
		},
	}
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "Rows per page, server clamps to its maximum")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "Pagination token returned by the previous call")
	return cmd
}

func runResultPreview(row runResultRow) string {
	inlineTexts := make([]string, 0)
	hasAccessURL := false
	for _, artifact := range row.Artifacts {
		if strings.TrimSpace(artifact.InlineText) != "" {
			inlineTexts = append(inlineTexts, oneLine(artifact.InlineText))
		}
		if strings.TrimSpace(artifact.AccessURL) != "" {
			hasAccessURL = true
		}
	}
	if len(inlineTexts) > 0 {
		preview := strings.Join(inlineTexts, " | ")
		if len(preview) > 160 {
			return preview[:157] + "..."
		}
		return preview
	}
	if hasAccessURL {
		return "download_url_available"
	}
	if strings.TrimSpace(row.Error) != "" {
		return oneLine(row.Error)
	}
	return "-"
}

func newRunResultWorkbookCmd(opts *rootOptions) *cobra.Command {
	var (
		outputPath      string
		downloadTimeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "result-workbook <run-id>",
		Short: "Download the server-generated workbook containing original inputs and results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dlOpts := *opts
			dlOpts.timeout = downloadTimeout
			httpClient, err := newHTTPClient(&dlOpts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), downloadTimeout)
			defer cancel()

			runID := strings.TrimSpace(args[0])
			resp, err := httpClient.GetBinary(ctx, "/users/me/runs/"+runID+"/resultWorkbook")
			if err != nil {
				return err
			}

			filename := suggestedDownloadFilename(resp.ContentDisposition)
			if filename == "" {
				filename = "result-" + runID + ".xlsx"
			}
			targetPath, err := resolveFilePath(outputPath, filepath.Base(filename))
			if err != nil {
				return fmt.Errorf("resolve output file path: %w", err)
			}
			if err := os.WriteFile(targetPath, resp.Body, 0o644); err != nil {
				return fmt.Errorf("write result workbook: %w", err)
			}
			opts.debugf("run result workbook: downloaded run_id=%s filename=%s size_bytes=%d", runID, filepath.Base(targetPath), len(resp.Body))

			result := map[string]any{
				"runId":       runID,
				"path":        targetPath,
				"filename":    filename,
				"size":        len(resp.Body),
				"contentType": resp.ContentType,
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "run_id\t%s\npath\t%s\nsize\t%d\n", runID, targetPath, len(resp.Body))
			return err
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output-file", "f", "", "Output .xlsx path or target directory")
	cmd.Flags().DurationVar(&downloadTimeout, "download-timeout", 5*time.Minute, "Timeout for the workbook download")
	return cmd
}
