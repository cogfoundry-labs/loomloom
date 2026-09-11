package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type subscriptionBundleSummary struct {
	ID              string  `json:"id"`
	SourceListingID *string `json:"sourceListingId,omitempty"`
	Name            string  `json:"name"`
	Status          string  `json:"status"`
	Revision        int64   `json:"revision"`
}

type subscriptionBundleList struct {
	Items         []subscriptionBundleSummary `json:"items"`
	NextPageToken string                      `json:"nextPageToken,omitempty"`
}

type subscriptionSummary struct {
	ID       string `json:"id"`
	BundleID string `json:"bundleId"`
	State    string `json:"state"`
}

func newSubscriptionCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "subscription", Short: "Manage SkillBot bundle subscriptions"}
	cmd.AddCommand(
		newSubscriptionListCmd(opts),
		newSubscriptionShowCmd(opts),
		newSubscriptionItemsCmd(opts),
		newSubscriptionBuyCmd(opts),
		newSubscriptionRenewCmd(opts),
	)
	return cmd
}

func newSubscriptionListCmd(opts *rootOptions) *cobra.Command {
	var pageSize int
	var pageToken string
	cmd := &cobra.Command{Use: "list", Short: "List my bundle subscriptions", RunE: func(cmd *cobra.Command, _ []string) error {
		query := subscriptionPageQuery(pageSize, pageToken)
		return subscriptionGetAndPrint(cmd, opts, "/users/me/subscriptions", query)
	}}
	addSubscriptionPageFlags(cmd, &pageSize, &pageToken)
	return cmd
}

func newSubscriptionShowCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "show <subscription-id>", Short: "Show one bundle subscription", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return subscriptionGetAndPrint(cmd, opts, "/users/me/subscriptions/"+url.PathEscape(strings.TrimSpace(args[0])), nil)
	}}
}

func newSubscriptionItemsCmd(opts *rootOptions) *cobra.Command {
	var snapshot bool
	var pageSize int
	var pageToken string
	cmd := &cobra.Command{Use: "items <subscription-id>", Short: "List current or purchase-snapshot SkillBots", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		suffix := "/items"
		if snapshot {
			suffix = "/snapshotItems"
		}
		return subscriptionGetAndPrint(
			cmd, opts, "/users/me/subscriptions/"+url.PathEscape(strings.TrimSpace(args[0]))+suffix,
			subscriptionPageQuery(pageSize, pageToken),
		)
	}}
	cmd.Flags().BoolVar(&snapshot, "snapshot", false, "Show the immutable purchase snapshot instead of current bundle members")
	addSubscriptionPageFlags(cmd, &pageSize, &pageToken)
	return cmd
}

func newSubscriptionBuyCmd(opts *rootOptions) *cobra.Command {
	return newSubscriptionPurchaseCmd(opts, false)
}

func newSubscriptionRenewCmd(opts *rootOptions) *cobra.Command {
	return newSubscriptionPurchaseCmd(opts, true)
}

func newSubscriptionPurchaseCmd(opts *rootOptions, renewal bool) *cobra.Command {
	var tier string
	var expectedRevision int64
	var clientRequestID string
	var confirm bool
	use := "buy <bundle-id>"
	short := "Buy a SkillBot bundle subscription"
	if renewal {
		use = "renew <subscription-id>"
		short = "Renew an expired SkillBot bundle subscription"
	}
	cmd := &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		tier = strings.TrimSpace(tier)
		if tier != "monthly" && tier != "yearly" {
			return fmt.Errorf("--tier must be monthly or yearly")
		}
		httpClient, err := newHTTPClient(opts)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
		defer cancel()

		resourceID := strings.TrimSpace(args[0])
		bundleID := resourceID
		var renewalOf *string
		if renewal {
			var existing subscriptionSummary
			if err := httpClient.GetProductJSON(ctx, "/users/me/subscriptions/"+url.PathEscape(resourceID), &existing); err != nil {
				return err
			}
			if strings.TrimSpace(existing.BundleID) == "" {
				return fmt.Errorf("subscription response has no bundleId")
			}
			bundleID = existing.BundleID
			renewalOf = &resourceID
		}
		if !confirm {
			var preview map[string]any
			if err := httpClient.GetProductJSON(ctx, "/subscriptionBundles/"+url.PathEscape(bundleID), &preview); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), map[string]any{"confirmed": false, "selectedTier": tier, "bundle": preview})
		}
		if expectedRevision <= 0 {
			return fmt.Errorf("--expected-revision must be greater than 0 when --confirm is set")
		}
		clientRequestID = strings.TrimSpace(clientRequestID)
		if clientRequestID == "" {
			return fmt.Errorf("--client-request-id is required when --confirm is set")
		}
		payload := map[string]any{"bundleId": bundleID, "tierType": tier, "clientRequestId": clientRequestID, "expectedRevision": expectedRevision}
		if renewalOf != nil {
			payload["renewalOf"] = *renewalOf
		}
		var response map[string]any
		if err := httpClient.PostProductJSON(ctx, "/users/me/subscriptions", payload, &response); err != nil {
			return err
		}
		return writeIndentedJSON(cmd.OutOrStdout(), response)
	}}
	cmd.Flags().StringVar(&tier, "tier", "", "Subscription tier: monthly|yearly")
	cmd.Flags().Int64Var(&expectedRevision, "expected-revision", 0, "Bundle revision shown during price review")
	cmd.Flags().StringVar(&clientRequestID, "client-request-id", "", "Stable idempotency key for this purchase")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm the displayed price and submit the purchase")
	_ = cmd.MarkFlagRequired("tier")
	return cmd
}

func subscriptionGetAndPrint(cmd *cobra.Command, opts *rootOptions, path string, query url.Values) error {
	httpClient, err := newHTTPClient(opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
	defer cancel()
	var response map[string]any
	if err := httpClient.GetProductJSONWithQuery(ctx, path, query, &response); err != nil {
		return err
	}
	return writeIndentedJSON(cmd.OutOrStdout(), response)
}

func subscriptionPageQuery(pageSize int, pageToken string) url.Values {
	query := url.Values{}
	if pageSize > 0 {
		query.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken = strings.TrimSpace(pageToken); pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	return query
}

func addSubscriptionPageFlags(cmd *cobra.Command, pageSize *int, pageToken *string) {
	cmd.Flags().IntVar(pageSize, "page-size", 20, "Page size, from 1 to 100")
	cmd.Flags().StringVar(pageToken, "page-token", "", "Opaque pagination token returned by the previous response")
}
