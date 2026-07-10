package cmd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// creatorTransactionSummary mirrors the backend creatorMarketTransactionResponse
// returned by /creators/me/marketTransactions. It intentionally omits
// creatorNetEarningT and creatorEarning: those are creator-earnings-specific
// fields shown only via the `creator earnings` command.
type creatorTransactionSummary struct {
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

type creatorTransactionsListResponse struct {
	Items      []creatorTransactionSummary `json:"items"`
	TotalCount int                         `json:"totalCount"`
}

func newCreatorCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "creator",
		Short: "Creator Market commands",
	}
	cmd.AddCommand(
		newCreatorEarningsCmd(opts),
		newCreatorTransactionsCmd(opts),
		newCreatorReviewCmd(opts),
	)
	return cmd
}

func newCreatorEarningsCmd(opts *rootOptions) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "earnings",
		Short: "List creator Market earnings",
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			query := url.Values{}
			if limit > 0 {
				query.Set("pageSize", fmt.Sprintf("%d", limit))
			}

			var resp map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/creators/me/earnings", query, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of earning records")
	return cmd
}

func newCreatorTransactionsCmd(opts *rootOptions) *cobra.Command {
	var pageSize int
	cmd := &cobra.Command{
		Use:   "transactions",
		Short: "List creator Market transactions",
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
			if err := httpClient.GetProductJSONWithQuery(ctx, "/creators/me/marketTransactions", query, &raw); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), raw)
			}
			resp, err := decodeJSONValue[creatorTransactionsListResponse](raw)
			if err != nil {
				return err
			}
			return printCreatorTransactions(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Page size")
	return cmd
}

func printCreatorTransactions(w io.Writer, resp creatorTransactionsListResponse) error {
	if len(resp.Items) == 0 {
		_, err := fmt.Fprintln(w, "no creator transactions")
		return err
	}
	tw := newTabWriter(w)
	if _, err := fmt.Fprintln(tw, "run_transaction_id\tlisting_id\tskill_name\ttask_fixed_fee\tfinal_payable\tstatus"); err != nil {
		return err
	}
	for _, item := range resp.Items {
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			item.RunTransactionID,
			item.ListingID,
			oneLine(item.SkillName),
			formatMoneyT(int64(item.TaskFixedFeeT), item.Currency),
			formatMoneyT(int64(item.FinalBuyerPayableT), item.Currency),
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

func newCreatorReviewCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Creator Market review request commands",
	}
	cmd.AddCommand(
		newCreatorReviewListCmd(opts),
		newCreatorReviewGetCmd(opts),
		newCreatorReviewWithdrawCmd(opts),
	)
	return cmd
}

func newCreatorReviewListCmd(opts *rootOptions) *cobra.Command {
	var (
		status   string
		pageSize int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List my Market review requests",
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

			var resp map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/creators/me/marketReviewRequests", query, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by review status")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Page size")
	return cmd
}

func newCreatorReviewGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <review-request-id>",
		Short: "Get one Market review request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			path := "/creators/me/marketReviewRequests/" + url.PathEscape(strings.TrimSpace(args[0]))
			var resp map[string]any
			if err := httpClient.GetProductJSON(ctx, path, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
}

func newCreatorReviewWithdrawCmd(opts *rootOptions) *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "withdraw <review-request-id>",
		Short: "Withdraw a pending Market review request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			payload := map[string]any{}
			if strings.TrimSpace(reason) != "" {
				payload["reason"] = strings.TrimSpace(reason)
			}

			path := "/creators/me/marketReviewRequests/" + url.PathEscape(strings.TrimSpace(args[0])) + ":withdraw"
			var resp map[string]any
			if err := httpClient.PostProductJSON(ctx, path, payload, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Optional withdrawal reason")
	return cmd
}
