package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// marketUsageSummary mirrors the backend marketUsageResponse returned by
// /users/me/marketUsages endpoints.
type marketUsageSummary struct {
	RunTransactionID        string         `json:"runTransactionId"`
	RunID                   string         `json:"runId"`
	ListingID               string         `json:"listingId"`
	ListingVersionID        string         `json:"listingVersionId"`
	SkillName               string         `json:"skillName"`
	TaskFixedFeeT           *flexInt64     `json:"taskFixedFeeT,omitempty"`
	TaskFixedFee            *moneyResponse `json:"taskFixedFee,omitempty"`
	EstimatedExecutionCostT *flexInt64     `json:"estimatedExecutionCostT,omitempty"`
	EstimatedExecutionCost  *moneyResponse `json:"estimatedExecutionCost,omitempty"`
	EstimatedBuyerPayableT  *flexInt64     `json:"estimatedBuyerPayableT,omitempty"`
	EstimatedBuyerPayable   *moneyResponse `json:"estimatedBuyerPayable,omitempty"`
	ActualExecutionCostT    *flexInt64     `json:"actualExecutionCostT,omitempty"`
	ActualExecutionCost     *moneyResponse `json:"actualExecutionCost,omitempty"`
	FinalBuyerPayableT      *flexInt64     `json:"finalBuyerPayableT,omitempty"`
	FinalBuyerPayable       *moneyResponse `json:"finalBuyerPayable,omitempty"`
	Currency                string         `json:"currency"`
	TransactionStatus       string         `json:"transactionStatus"`
}

type marketUsagesListResponse struct {
	Items      []marketUsageSummary `json:"items"`
	TotalCount int                  `json:"totalCount"`
}

// Keep raw amounts exact while preserving unknown response fields for JSON
// output. Converting through float64 can corrupt the derived creator call fee.
type usageResponse map[string]any

func (r *usageResponse) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode((*map[string]any)(r))
}

func newUsageCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Buyer Market usage commands",
	}
	cmd.AddCommand(
		newUsageListCmd(opts),
		newUsageGetCmd(opts),
	)
	return cmd
}

func newUsageListCmd(opts *rootOptions) *cobra.Command {
	var pageSize int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List my Market SkillApp usage records",
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

			var raw usageResponse
			if err := httpClient.GetProductJSONWithQuery(ctx, "/users/me/marketUsages", query, &raw); err != nil {
				return err
			}
			if opts.output == "json" {
				if items, ok := raw["items"].([]any); ok {
					for i, item := range items {
						record, ok := item.(map[string]any)
						if !ok {
							return fmt.Errorf("invalid usage record at index %d", i)
						}
						display, err := usageDisplayPayload(record)
						if err != nil {
							return err
						}
						items[i] = display
					}
				}
				return writeExecutionJSON(cmd.OutOrStdout(), raw)
			}
			resp, err := decodeJSONValue[marketUsagesListResponse](raw)
			if err != nil {
				return err
			}
			return printUsageList(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Page size")
	return cmd
}

func newUsageGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <run-transaction-id>",
		Short: "Get one Market SkillApp usage record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			path := "/users/me/marketUsages/" + url.PathEscape(strings.TrimSpace(args[0]))
			var raw usageResponse
			if err := httpClient.GetProductJSON(ctx, path, &raw); err != nil {
				return err
			}
			if opts.output == "json" {
				display, err := usageDisplayPayload(raw)
				if err != nil {
					return err
				}
				return writeExecutionJSON(cmd.OutOrStdout(), display)
			}
			resp, err := decodeJSONValue[marketUsageSummary](raw)
			if err != nil {
				return err
			}
			return printUsageDetail(cmd.OutOrStdout(), resp)
		},
	}
}

func printUsageList(w io.Writer, resp marketUsagesListResponse) error {
	if len(resp.Items) == 0 {
		_, err := fmt.Fprintln(w, "no market usage records")
		return err
	}
	tw := newTabWriter(w)
	if _, err := fmt.Fprintln(tw, "run_transaction_id\tlisting_id\tskill_name\tactual_amount\tcreator_call_fee\tmodel_fee\tstatus"); err != nil {
		return err
	}
	for _, item := range resp.Items {
		fees, err := usageActualFees(item)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			item.RunTransactionID,
			item.ListingID,
			oneLine(item.SkillName),
			fees.format(fees.total),
			fees.format(fees.creator),
			fees.format(fees.model),
			oneLine(item.TransactionStatus),
		); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "total_count\t%d\n", resp.TotalCount)
	return err
}

func printUsageDetail(w io.Writer, usage marketUsageSummary) error {
	fees, err := usageActualFees(usage)
	if err != nil {
		return err
	}
	tw := newTabWriter(w)
	for _, row := range [][2]string{
		{"run_transaction_id", usage.RunTransactionID},
		{"run_id", usage.RunID},
		{"listing_id", usage.ListingID},
		{"listing_version_id", usage.ListingVersionID},
		{"skill_name", usage.SkillName},
		{"currency", usage.Currency},
	} {
		if row[1] == "" {
			continue
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", row[0], oneLine(row[1])); err != nil {
			return err
		}
	}
	for _, row := range [][2]string{
		{"actual_amount", fees.format(fees.total)},
		{"creator_call_fee", fees.format(fees.creator)},
		{"model_fee", fees.format(fees.model)},
	} {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}
	if usage.TransactionStatus != "" {
		if _, err := fmt.Fprintf(tw, "transaction_status\t%s\n", oneLine(usage.TransactionStatus)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

type usageFees struct {
	status  string
	total   *moneyResponse
	creator *moneyResponse
	model   *moneyResponse
}

func (f usageFees) format(money *moneyResponse) string {
	if money != nil {
		return money.Currency + " " + money.Amount
	}
	if f.status == "pending" {
		return "pending settlement"
	}
	return "unavailable"
}

func usageActualFees(usage marketUsageSummary) (usageFees, error) {
	fees := usageFees{status: "pending"}
	switch strings.ToLower(strings.TrimSpace(usage.TransactionStatus)) {
	case "succeeded", "failed", "cancelled", "settled":
		// Only a terminal transaction has final amounts. A terminal run alone
		// is insufficient: the Market worker may not have reconciled it yet.
	default:
		return fees, nil
	}
	fees.status = "unavailable"
	total, totalT, err := usageMoney(usage.FinalBuyerPayable, usage.FinalBuyerPayableT, usage.Currency)
	if err != nil {
		return fees, fmt.Errorf("usage %s finalBuyerPayable contract error: %w", usage.RunTransactionID, err)
	}
	model, modelT, err := usageMoney(usage.ActualExecutionCost, usage.ActualExecutionCostT, usage.Currency)
	if err != nil {
		return fees, fmt.Errorf("usage %s actualExecutionCost contract error: %w", usage.RunTransactionID, err)
	}
	fees.total, fees.model = total, model
	if total == nil || model == nil {
		return fees, nil
	}
	if total.Currency != model.Currency {
		return fees, fmt.Errorf("usage %s actual fee currencies do not match", usage.RunTransactionID)
	}
	if totalT < modelT {
		return fees, fmt.Errorf("usage %s final payable is less than actual model cost", usage.RunTransactionID)
	}
	// Market fulfillment persists final payable = actual model cost + creator
	// call fee for completed tasks. Do not multiply the quoted per-task rate:
	// subscription coverage and partial completion can change the charged fee.
	fees.creator = &moneyResponse{Amount: decimalAmountT(totalT - modelT), Currency: total.Currency}
	fees.status = "available"
	return fees, nil
}

func usageMoney(money *moneyResponse, raw *flexInt64, currency string) (*moneyResponse, int64, error) {
	if money == nil && raw == nil {
		return nil, 0, nil
	}
	if _, err := formatResponseMoney(money, raw, currency); err != nil {
		return nil, 0, err
	}
	var amountT int64
	if money != nil {
		amountT, _ = parseMoneyAmountT(money.Amount) // Validated by formatResponseMoney.
		currency = money.Currency
	} else {
		amountT = int64(*raw)
	}
	if amountT < 0 {
		return nil, 0, fmt.Errorf("actual fee must be non-negative")
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if !isCurrencyCode(currency) {
		return nil, 0, nil
	}
	return &moneyResponse{Amount: decimalAmountT(amountT), Currency: currency}, amountT, nil
}

func usageDisplayPayload(raw map[string]any) (map[string]any, error) {
	usage, err := decodeJSONValue[marketUsageSummary](raw)
	if err != nil {
		return nil, err
	}
	fees, err := usageActualFees(usage)
	if err != nil {
		return nil, err
	}
	display := make(map[string]any, len(raw)+2)
	for key, value := range raw {
		display[key] = value
	}
	// Preserve the receiving DTO and unknown metadata; only reshape user output.
	for _, key := range []string{"taskFixedFee", "taskFixedFeeT", "actualExecutionCostT", "finalBuyerPayableT"} {
		delete(display, key)
	}
	display["feeStatus"] = fees.status
	display["finalBuyerPayable"] = fees.total
	display["actualCreatorFee"] = fees.creator
	display["actualExecutionCost"] = fees.model
	return display, nil
}
