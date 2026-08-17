package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type flexInt64 int64

func (v *flexInt64) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*v = 0
		return nil
	}
	if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
		trimmed = strings.Trim(trimmed, "\"")
	}
	var parsed int64
	if _, err := fmt.Sscan(trimmed, &parsed); err != nil {
		return fmt.Errorf("parse int64 %q: %w", trimmed, err)
	}
	*v = flexInt64(parsed)
	return nil
}

type flexInt int

func (v *flexInt) UnmarshalJSON(data []byte) error {
	var parsed flexInt64
	if err := parsed.UnmarshalJSON(data); err != nil {
		return err
	}
	*v = flexInt(parsed)
	return nil
}

type moneyResponse struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type templateDisplayRow struct {
	Values map[string]string `json:"values"`
}

type rowValidationError struct {
	RowIndex int    `json:"rowIndex"`
	FieldKey string `json:"fieldKey"`
	Error    string `json:"error"`
}

type validateTemplateRowsResponse struct {
	Valid     bool                 `json:"valid"`
	RowErrors []rowValidationError `json:"rowErrors"`
}

type templateBalanceCheck struct {
	Currency              string         `json:"currency"`
	AvailableBalance      *flexInt64     `json:"availableBalance,omitempty"`
	AvailableBalanceMoney *moneyResponse `json:"availableBalanceMoney,omitempty"`
	IsSufficient          bool           `json:"isSufficient"`
}

type userBalanceSnapshotResponse struct {
	Currency         string         `json:"currency"`
	AvailableBalance *flexInt64     `json:"availableBalanceT,omitempty"`
	AvailableMoney   *moneyResponse `json:"availableBalance,omitempty"`
}

type precheckTemplateRowsResponse struct {
	EstimatedTotalCostT *flexInt64            `json:"estimatedTotalCostT,omitempty"`
	EstimatedTotalCost  *moneyResponse        `json:"estimatedTotalCost,omitempty"`
	BalanceCheck        *templateBalanceCheck `json:"balanceCheck"`
}

type submitTemplateRowsResponse struct {
	RunID      string    `json:"runId"`
	Status     string    `json:"status"`
	AcceptedAt flexInt64 `json:"acceptedAt"`
}

func (r *submitTemplateRowsResponse) UnmarshalJSON(data []byte) error {
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

type runStatusResponse struct {
	RunID             string         `json:"runId"`
	Status            string         `json:"status"`
	DefinitionHash    string         `json:"definitionHash"`
	ErrorMessage      string         `json:"errorMessage"`
	FirstErrorMessage string         `json:"firstErrorMessage"`
	TotalTasks        flexInt        `json:"totalTasks"`
	CompletedTasks    flexInt        `json:"completedTasks"`
	FailedTasks       flexInt        `json:"failedTasks"`
	CancelledTasks    flexInt        `json:"cancelledTasks"`
	EstimatedCostT    *flexInt64     `json:"estimatedCostT,omitempty"`
	EstimatedCost     *moneyResponse `json:"estimatedCost,omitempty"`
	ActualCostT       *flexInt64     `json:"actualCostT,omitempty"`
	ActualCost        *moneyResponse `json:"actualCost,omitempty"`
	StartedAtUnix     flexInt64      `json:"startedAtUnix"`
	CompletedAtUnix   flexInt64      `json:"completedAtUnix"`
}

type runGetResponse struct {
	Run runStatusResponse `json:"run"`
}

type artifactEntry struct {
	ArtifactID     string    `json:"artifactId"`
	TaskID         string    `json:"taskId"`
	StepID         string    `json:"stepId"`
	MimeType       string    `json:"mimeType"`
	PortName       string    `json:"portName"`
	AccessURL      string    `json:"accessUrl"`
	InlineText     string    `json:"inlineText"`
	CreatedAtUnix  flexInt64 `json:"createdAtUnix"`
	SourceRowIndex flexInt   `json:"sourceRowIndex"`
}

type runTaskEntry struct {
	TaskID         string  `json:"taskId"`
	SourceRowIndex flexInt `json:"sourceRowIndex"`
	Status         string  `json:"status"`
	ErrorMessage   string  `json:"errorMessage"`
	ArtifactCount  flexInt `json:"artifactCount"`
}

type listRunTasksResponse struct {
	Tasks         []runTaskEntry `json:"tasks"`
	NextPageToken string         `json:"nextPageToken"`
	TotalCount    int            `json:"totalCount"`
}

type listRunArtifactsResponse struct {
	Artifacts     []artifactEntry `json:"artifacts"`
	NextPageToken string          `json:"nextPageToken"`
	TotalCount    int             `json:"totalCount"`
}

func (r *listRunArtifactsResponse) UnmarshalJSON(data []byte) error {
	type alias struct {
		Artifacts     []artifactEntry `json:"artifacts"`
		Items         []artifactEntry `json:"items"`
		NextPageToken string          `json:"nextPageToken"`
		TotalCount    int             `json:"totalCount"`
	}
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	r.Artifacts = parsed.Artifacts
	if len(r.Artifacts) == 0 {
		r.Artifacts = parsed.Items
	}
	r.NextPageToken = parsed.NextPageToken
	r.TotalCount = parsed.TotalCount
	return nil
}

func newTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
}

func writeIndentedJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func readJSONFileMap(filePath string) (map[string]any, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, errors.New("--input-file is required")
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read input file: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("input file is empty")
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse input JSON: %w", err)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}

func removeIdentityFields(payload map[string]any) {
	for _, field := range []string{
		"user_id",
		"buyer_user_id",
		"creator_user_id",
		"userId",
		"buyerUserId",
		"creatorUserId",
	} {
		delete(payload, field)
	}
}

func effectiveClientRequestID(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value != "" {
		return value, false
	}
	return fmt.Sprintf("loomloom-cli-%d", time.Now().UnixNano()), true
}

func printGeneratedClientRequestID(cmd *cobra.Command, value string, generated bool) {
	if generated {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "clientRequestId: %s\n", value)
	}
}

func stringMapValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return "-"
	}
	trimmed := strings.TrimSpace(fmt.Sprint(value))
	if trimmed == "" {
		return "-"
	}
	return trimmed
}

// int64MapValue reads an integer-valued field from a decoded JSON map. JSON
// numbers decode as float64, so this also tolerates a string encoding.
func int64MapValue(values map[string]any, key string) (int64, bool) {
	value, ok := values[key]
	if !ok || value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case int16:
		return int64(v), true
	case int8:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case json.Number:
		parsed, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		var parsed int64
		if _, err := fmt.Sscan(trimmed, &parsed); err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func formatUnix(ts int64) string {
	if ts <= 0 {
		return "-"
	}
	return time.Unix(ts, 0).Format(time.RFC3339)
}

func formatDuration(startUnix int64, endUnix int64) string {
	if startUnix <= 0 || endUnix <= 0 || endUnix < startUnix {
		return "-"
	}
	return time.Unix(endUnix, 0).Sub(time.Unix(startUnix, 0)).String()
}

func formatCost(cost int64) string {
	return formatMoneyT(cost, "")
}

func formatMoney(amountT int64, currency string) string {
	return formatMoneyT(amountT, currency)
}

// formatMoneyT converts a raw *T amount (10,000,000 T = 1 currency unit) into
// a human-readable string at 7-decimal precision. When currency is empty, it
// does not guess a currency and instead marks the amount as unknown.
func formatMoneyT(amountT int64, currency string) string {
	currency = strings.TrimSpace(currency)
	sign := ""
	displayAmount := amountT
	if displayAmount < 0 {
		sign = "-"
		displayAmount = -displayAmount
	}
	value := new(big.Rat).SetFrac(big.NewInt(displayAmount), big.NewInt(10_000_000))
	if currency == "" {
		return fmt.Sprintf("(currency unknown) %d", amountT)
	}
	return strings.ToUpper(currency) + " " + sign + value.FloatString(7)
}

func formatResponseMoney(money *moneyResponse, amountT *flexInt64, fallbackCurrency string) (string, error) {
	if money == nil {
		rawAmountT := int64(0)
		if amountT != nil {
			rawAmountT = int64(*amountT)
		}
		return formatMoneyT(rawAmountT, fallbackCurrency), nil
	}

	amount := strings.TrimSpace(money.Amount)
	parsedAmountT, err := parseMoneyAmountT(amount)
	if err != nil {
		return "", fmt.Errorf("invalid money amount %q: %w", money.Amount, err)
	}
	if amountT != nil && parsedAmountT != int64(*amountT) {
		return "", fmt.Errorf(
			"money amount %s does not match raw amount %d",
			amount,
			int64(*amountT),
		)
	}

	currency := strings.ToUpper(strings.TrimSpace(money.Currency))
	if !isCurrencyCode(currency) {
		return "", fmt.Errorf("invalid money currency %q", money.Currency)
	}
	fallbackCurrency = strings.ToUpper(strings.TrimSpace(fallbackCurrency))
	if fallbackCurrency != "" && currency != fallbackCurrency {
		return "", fmt.Errorf(
			"money currency %s does not match response currency %s",
			currency,
			fallbackCurrency,
		)
	}
	return currency + " " + amount, nil
}

func isCurrencyCode(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for _, r := range currency {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func parseMoneyAmountT(raw string) (int64, error) {
	const unitsPerCurrency = int64(10_000_000)
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, errors.New("amount is required")
	}
	if strings.HasPrefix(value, "-") {
		return 0, errors.New("amount must be non-negative")
	}
	if strings.HasPrefix(value, "+") {
		value = strings.TrimPrefix(value, "+")
	}
	if strings.ContainsAny(value, "eE/,") {
		return 0, fmt.Errorf("invalid amount %q; use a decimal value such as 0.5", raw)
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid amount %q; use a decimal value such as 0.5", raw)
	}
	whole := parts[0]
	fractional := ""
	if len(parts) == 2 {
		fractional = parts[1]
	}
	if whole == "" && fractional == "" {
		return 0, fmt.Errorf("invalid amount %q; use a decimal value such as 0.5", raw)
	}
	if whole == "" {
		whole = "0"
	}
	if !allDigits(whole) || (fractional != "" && !allDigits(fractional)) {
		return 0, fmt.Errorf("invalid amount %q; use a decimal value such as 0.5", raw)
	}
	fractional = strings.TrimRight(fractional, "0")
	if len(fractional) > 7 {
		return 0, errors.New("amount must be a multiple of 0.0000001")
	}

	total := new(big.Int)
	wholeInt := new(big.Int)
	if _, ok := wholeInt.SetString(whole, 10); !ok {
		return 0, fmt.Errorf("invalid amount %q; use a decimal value such as 0.5", raw)
	}
	total.Mul(wholeInt, big.NewInt(unitsPerCurrency))
	if fractional != "" {
		padded := fractional + strings.Repeat("0", 7-len(fractional))
		fractionalInt := new(big.Int)
		if _, ok := fractionalInt.SetString(padded, 10); !ok {
			return 0, fmt.Errorf("invalid amount %q; use a decimal value such as 0.5", raw)
		}
		total.Add(total, fractionalInt)
	}
	if !total.IsInt64() {
		return 0, fmt.Errorf("amount %q is too large", raw)
	}
	return total.Int64(), nil
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isTerminalRunStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "partially_failed", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func printValidation(w io.Writer, resp validateTemplateRowsResponse) error {
	if resp.Valid {
		_, err := fmt.Fprintln(w, "valid")
		return err
	}
	if _, err := fmt.Fprintln(w, "invalid"); err != nil {
		return err
	}
	for _, rowErr := range resp.RowErrors {
		if _, err := fmt.Fprintf(w, "row %d field %s: %s\n", rowErr.RowIndex, rowErr.FieldKey, rowErr.Error); err != nil {
			return err
		}
	}
	return nil
}

func printTemplateFileValidation(w io.Writer, resp validateTemplateFileResponse) error {
	if resp.Valid {
		_, err := fmt.Fprintln(w, "valid")
		return err
	}
	if _, err := fmt.Fprintln(w, "invalid"); err != nil {
		return err
	}
	for _, fileErr := range resp.FileErrors {
		if _, err := fmt.Fprintf(w, "file: %s\n", fileErr); err != nil {
			return err
		}
	}
	for _, rowErr := range resp.RowErrors {
		if _, err := fmt.Fprintf(w, "row %d field %s: %s\n", rowErr.RowIndex, rowErr.FieldKey, rowErr.Error); err != nil {
			return err
		}
	}
	return nil
}

func printPrecheck(w io.Writer, resp precheckTemplateRowsResponse) error {
	currency := ""
	if resp.BalanceCheck != nil && strings.TrimSpace(resp.BalanceCheck.Currency) != "" {
		currency = resp.BalanceCheck.Currency
	}
	tw := newTabWriter(w)
	estimatedCost, err := formatResponseMoney(resp.EstimatedTotalCost, resp.EstimatedTotalCostT, currency)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "estimated_cost\t%s\n", estimatedCost); err != nil {
		return err
	}
	if resp.BalanceCheck == nil {
		return tw.Flush()
	}
	availableBalance, err := formatSignedBalanceMoney(
		resp.BalanceCheck.AvailableBalanceMoney,
		resp.BalanceCheck.AvailableBalance,
		currency,
	)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "available_balance\t%s\n", availableBalance); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "sufficient\t%t\n", resp.BalanceCheck.IsSufficient); err != nil {
		return err
	}
	return tw.Flush()
}

func precheckJSONPayload(resp precheckTemplateRowsResponse) map[string]any {
	result := map[string]any{
		"balanceCheck": resp.BalanceCheck,
	}
	if resp.EstimatedTotalCostT != nil {
		result["estimatedTotalCostT"] = int64(*resp.EstimatedTotalCostT)
	}
	if resp.EstimatedTotalCost != nil {
		result["estimatedTotalCost"] = resp.EstimatedTotalCost
	}
	return result
}

func printRunSummary(w io.Writer, resp runStatusResponse) error {
	if _, err := fmt.Fprintf(w, "run_id\t%s\nstatus\t%s\n", resp.RunID, resp.Status); err != nil {
		return err
	}
	if resp.DefinitionHash != "" {
		if _, err := fmt.Fprintf(w, "definition_hash\t%s\n", resp.DefinitionHash); err != nil {
			return err
		}
	}
	if resp.ErrorMessage != "" {
		if _, err := fmt.Fprintf(w, "error\t%s\n", resp.ErrorMessage); err != nil {
			return err
		}
	}
	if resp.FirstErrorMessage != "" && resp.FirstErrorMessage != resp.ErrorMessage {
		if _, err := fmt.Fprintf(w, "first_error\t%s\n", resp.FirstErrorMessage); err != nil {
			return err
		}
	}
	tw := newTabWriter(w)
	if _, err := fmt.Fprintln(tw, "total\tcompleted\tfailed\tcancelled\tcost\tstarted_at\tcompleted_at\tduration"); err != nil {
		return err
	}
	actualCost, err := formatResponseMoney(resp.ActualCost, resp.ActualCostT, "")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		tw,
		"%d\t%d\t%d\t%d\t%s\t%s\t%s\t%s\n",
		int(resp.TotalTasks),
		int(resp.CompletedTasks),
		int(resp.FailedTasks),
		int(resp.CancelledTasks),
		actualCost,
		formatUnix(int64(resp.StartedAtUnix)),
		formatUnix(int64(resp.CompletedAtUnix)),
		formatDuration(int64(resp.StartedAtUnix), int64(resp.CompletedAtUnix)),
	); err != nil {
		return err
	}
	return tw.Flush()
}

func printInputAssetUpload(w io.Writer, resp uploadInputAssetResponse) error {
	if _, err := fmt.Fprintf(w, "input_asset_id\t%s\n", resp.InputAssetID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "filename\t%s\n", resp.Filename); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "mime_type\t%s\n", resp.MimeType); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "size_bytes\t%d\n", int64(resp.SizeBytes)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "uploaded_at\t%s\n", formatUnix(int64(resp.UploadedAt))); err != nil {
		return err
	}
	return nil
}

func printOrchestrationInputUpload(w io.Writer, resp uploadOrchestrationInputResponse) error {
	if _, err := fmt.Fprintf(w, "input_file_id\t%s\n", resp.InputFileID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "filename\t%s\n", resp.Filename); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "row_count\t%d\n", resp.RowCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "uploaded_at\t%s\n", formatUnix(int64(resp.UploadedAt))); err != nil {
		return err
	}
	return nil
}

func printArtifacts(w io.Writer, artifacts []artifactEntry) error {
	tw := newTabWriter(w)
	if _, err := fmt.Fprintln(tw, "artifact_id\ttask_id\tstep_id\tmime_type\tport\trow\tcreated_at\taccess"); err != nil {
		return err
	}
	for _, art := range artifacts {
		access := art.AccessURL
		if access == "" && art.InlineText != "" {
			access = "inline_text"
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			art.ArtifactID,
			art.TaskID,
			art.StepID,
			art.MimeType,
			art.PortName,
			int(art.SourceRowIndex),
			formatUnix(int64(art.CreatedAtUnix)),
			access,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func loadTemplateRows(path string) ([]templateDisplayRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("input row file is empty")
	}
	if trimmed[0] == '[' {
		var rows []map[string]any
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return nil, fmt.Errorf("parse json array: %w", err)
		}
		return normalizeRows(rows)
	}

	scanner := bufio.NewScanner(bytes.NewReader(trimmed))
	rows := make([]map[string]any, 0)
	line := 0
	for scanner.Scan() {
		line++
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return nil, fmt.Errorf("parse jsonl line %d: %w", line, err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return normalizeRows(rows)
}

func normalizeRows(rows []map[string]any) ([]templateDisplayRow, error) {
	normalized := make([]templateDisplayRow, 0, len(rows))
	for idx, row := range rows {
		values := make(map[string]string, len(row))
		for key, rawValue := range row {
			value, err := scalarToString(rawValue)
			if err != nil {
				return nil, fmt.Errorf("row %d field %s: %w", idx, key, err)
			}
			values[key] = value
		}
		normalized = append(normalized, templateDisplayRow{Values: values})
	}
	return normalized, nil
}

func templateRowsPayload(rows []templateDisplayRow) []map[string]string {
	payloadRows := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		values := make(map[string]string, len(row.Values))
		for key, value := range row.Values {
			values[key] = value
		}
		payloadRows = append(payloadRows, values)
	}
	return payloadRows
}

func remapRowsToHeaderLabels(rows []templateDisplayRow, schema templateSchemaResponse) []templateDisplayRow {
	if len(rows) == 0 || len(schema.Columns) == 0 {
		return rows
	}

	labelByFieldKey := make(map[string]string, len(schema.Columns))
	knownLabels := make(map[string]struct{}, len(schema.Columns))
	for _, column := range schema.Columns {
		if column.FieldKey != "" && column.HeaderLabel != "" {
			labelByFieldKey[column.FieldKey] = column.HeaderLabel
			knownLabels[column.HeaderLabel] = struct{}{}
		}
	}

	remapped := make([]templateDisplayRow, 0, len(rows))
	for _, row := range rows {
		values := make(map[string]string, len(row.Values))
		for key, value := range row.Values {
			if label, ok := labelByFieldKey[key]; ok {
				values[label] = value
				continue
			}
			if _, ok := knownLabels[key]; ok {
				values[key] = value
				continue
			}
			values[key] = value
		}
		remapped = append(remapped, templateDisplayRow{Values: values})
	}
	return remapped
}

func scalarToString(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case bool, float64, float32, int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
		return fmt.Sprint(v), nil
	default:
		return "", fmt.Errorf("unsupported non-scalar value of type %T", value)
	}
}

func validationError(resp validateTemplateRowsResponse) error {
	if resp.Valid {
		return nil
	}
	if len(resp.RowErrors) == 0 {
		return errors.New("template rows validation failed")
	}
	first := resp.RowErrors[0]
	return fmt.Errorf("template rows validation failed: row %d field %s: %s", first.RowIndex, first.FieldKey, first.Error)
}

func templateFileValidationError(resp validateTemplateFileResponse) error {
	if resp.Valid {
		return nil
	}
	if len(resp.FileErrors) > 0 {
		return fmt.Errorf("template file validation failed: %s", resp.FileErrors[0])
	}
	if len(resp.RowErrors) > 0 {
		first := resp.RowErrors[0]
		return fmt.Errorf("template file validation failed: row %d field %s: %s", first.RowIndex, first.FieldKey, first.Error)
	}
	return errors.New("template file validation failed")
}

func resolveFilePath(target string, defaultName string) (string, error) {
	if strings.TrimSpace(target) == "" {
		target = defaultName
	}

	info, err := os.Stat(target)
	if err == nil && info.IsDir() {
		target = filepath.Join(target, defaultName)
	} else if err != nil && strings.HasSuffix(target, string(os.PathSeparator)) {
		if mkErr := os.MkdirAll(target, 0o755); mkErr != nil {
			return "", mkErr
		}
		target = filepath.Join(target, defaultName)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	return filepath.Abs(target)
}

func inferArtifactFilename(artifact artifactEntry) string {
	if artifact.AccessURL != "" {
		if parsed, err := neturl.Parse(artifact.AccessURL); err == nil {
			if base := path.Base(parsed.Path); base != "" && base != "." && base != "/" {
				return base
			}
		}
	}
	if artifact.InlineText != "" {
		return artifact.ArtifactID + ".txt"
	}
	if exts, _ := mime.ExtensionsByType(artifact.MimeType); len(exts) > 0 {
		return artifact.ArtifactID + exts[0]
	}
	return artifact.ArtifactID
}

func downloadURL(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(resp.Body)
}
