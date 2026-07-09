package cmd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// marketUsageSummary mirrors the backend marketUsageResponse returned by
// /users/me/marketUsages endpoints.
type marketUsageSummary struct {
	RunTransactionID        string    `json:"runTransactionId"`
	RunID                   string    `json:"runId"`
	ListingID               string    `json:"listingId"`
	ListingVersionID        string    `json:"listingVersionId"`
	SkillName               string    `json:"skillName"`
	TaskFixedFeeT           flexInt64 `json:"taskFixedFeeT"`
	EstimatedExecutionCostT flexInt64 `json:"estimatedExecutionCostT"`
	EstimatedBuyerPayableT  flexInt64 `json:"estimatedBuyerPayableT"`
	ActualExecutionCostT    flexInt64 `json:"actualExecutionCostT"`
	FinalBuyerPayableT      flexInt64 `json:"finalBuyerPayableT"`
	Currency                string    `json:"currency"`
	TransactionStatus       string    `json:"transactionStatus"`
}

type marketUsagesListResponse struct {
	Items      []marketUsageSummary `json:"items"`
	TotalCount int                  `json:"totalCount"`
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
		Short: "List my Market SkillBot usage records",
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

			var raw map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/users/me/marketUsages", query, &raw); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), raw)
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
		Short: "Get one Market SkillBot usage record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			path := "/users/me/marketUsages/" + url.PathEscape(strings.TrimSpace(args[0]))
			var raw map[string]any
			if err := httpClient.GetProductJSON(ctx, path, &raw); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), raw)
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
	if _, err := fmt.Fprintln(tw, "run_transaction_id\tlisting_id\tskill_name\ttask_fixed_fee\ttask_fixed_fee_t\tfinal_payable\tfinal_payable_t\tstatus"); err != nil {
		return err
	}
	for _, item := range resp.Items {
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%d\t%s\t%d\t%s\n",
			item.RunTransactionID,
			item.ListingID,
			oneLine(item.SkillName),
			formatMoneyT(int64(item.TaskFixedFeeT), item.Currency),
			int64(item.TaskFixedFeeT),
			formatMoneyT(int64(item.FinalBuyerPayableT), item.Currency),
			int64(item.FinalBuyerPayableT),
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
	for _, field := range []struct {
		label  string
		rawKey string
		amount int64
	}{
		{"task_fixed_fee", "task_fixed_fee_t", int64(usage.TaskFixedFeeT)},
		{"estimated_execution_cost", "estimated_execution_cost_t", int64(usage.EstimatedExecutionCostT)},
		{"estimated_payable", "estimated_payable_t", int64(usage.EstimatedBuyerPayableT)},
		{"actual_execution_cost", "actual_execution_cost_t", int64(usage.ActualExecutionCostT)},
		{"final_payable", "final_payable_t", int64(usage.FinalBuyerPayableT)},
	} {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", field.label, formatMoneyT(field.amount, usage.Currency)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "%s\t%d\n", field.rawKey, field.amount); err != nil {
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
