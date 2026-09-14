package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/client"
	"github.com/spf13/cobra"
)

func newMarketBundleCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "bundle", Short: "Browse published SkillBot subscription bundles"}
	cmd.AddCommand(newMarketBundleListCmd(opts), newMarketBundleShowCmd(opts), newMarketBundleItemsCmd(opts))
	return cmd
}

func newMarketBundleListCmd(opts *rootOptions) *cobra.Command {
	var creatorID, name, pageToken string
	var pageSize int
	cmd := &cobra.Command{Use: "list", Short: "List published subscription bundles", RunE: func(cmd *cobra.Command, _ []string) error {
		query := subscriptionPageQuery(pageSize, pageToken)
		if value := strings.TrimSpace(creatorID); value != "" {
			if parsed, err := strconv.ParseUint(value, 10, 64); err != nil || parsed == 0 {
				return fmt.Errorf("--creator-id must be a positive user ID")
			}
			query.Set("creatorId", value)
		}
		if value := strings.TrimSpace(name); value != "" {
			query.Set("name", value)
		}
		return subscriptionGetAndPrint(cmd, opts, "/subscriptionBundles", query)
	}}
	cmd.Flags().StringVar(&creatorID, "creator-id", "", "Filter by creator user ID")
	cmd.Flags().StringVar(&name, "name", "", "Filter by bundle name")
	addSubscriptionPageFlags(cmd, &pageSize, &pageToken)
	return cmd
}

func newMarketBundleShowCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "show <bundle-id>", Short: "Show a published subscription bundle", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return subscriptionGetAndPrint(cmd, opts, "/subscriptionBundles/"+url.PathEscape(strings.TrimSpace(args[0])), nil)
	}}
}

func newMarketBundleItemsCmd(opts *rootOptions) *cobra.Command {
	var pageSize int
	var pageToken string
	cmd := &cobra.Command{Use: "items <bundle-id>", Short: "List SkillBots in a published subscription bundle", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := "/subscriptionBundles/" + url.PathEscape(strings.TrimSpace(args[0])) + "/items"
		return subscriptionGetAndPrint(cmd, opts, path, subscriptionPageQuery(pageSize, pageToken))
	}}
	addSubscriptionPageFlags(cmd, &pageSize, &pageToken)
	return cmd
}

func newMarketSubscribeCmd(opts *rootOptions) *cobra.Command {
	var tier, clientRequestID string
	var expectedRevision int64
	var confirm bool
	cmd := &cobra.Command{Use: "subscribe <listing-id>", Short: "Buy the listing's automatic single-SkillBot subscription", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
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
		bundle, err := findAutomaticBundle(ctx, httpClient, strings.TrimSpace(args[0]))
		if err != nil {
			return err
		}
		if !confirm {
			var detail map[string]any
			if err := httpClient.GetProductJSON(ctx, "/subscriptionBundles/"+url.PathEscape(bundle.ID), &detail); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), map[string]any{"confirmed": false, "selectedTier": tier, "bundle": detail})
		}
		if expectedRevision <= 0 || expectedRevision != bundle.Revision {
			return fmt.Errorf("--expected-revision must equal the current automatic bundle revision %d", bundle.Revision)
		}
		clientRequestID = strings.TrimSpace(clientRequestID)
		if clientRequestID == "" {
			return fmt.Errorf("--client-request-id is required when --confirm is set")
		}
		var response map[string]any
		if err := httpClient.PostProductJSON(ctx, "/users/me/subscriptions", map[string]any{"bundleId": bundle.ID, "tierType": tier, "clientRequestId": clientRequestID, "expectedRevision": expectedRevision}, &response); err != nil {
			return err
		}
		return writeIndentedJSON(cmd.OutOrStdout(), response)
	}}
	cmd.Flags().StringVar(&tier, "tier", "", "Subscription tier: monthly|yearly")
	cmd.Flags().Int64Var(&expectedRevision, "expected-revision", 0, "Automatic bundle revision shown during price review")
	cmd.Flags().StringVar(&clientRequestID, "client-request-id", "", "Stable idempotency key for this purchase")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm the displayed price and submit the purchase")
	_ = cmd.MarkFlagRequired("tier")
	return cmd
}

func findAutomaticBundle(ctx context.Context, httpClient *client.Client, listingID string) (*subscriptionBundleSummary, error) {
	if listingID == "" {
		return nil, fmt.Errorf("listing ID is required")
	}
	pageToken := ""
	seen := map[string]struct{}{}
	for {
		query := url.Values{"pageSize": []string{"100"}}
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		var page subscriptionBundleList
		if err := httpClient.GetProductJSONWithQuery(ctx, "/subscriptionBundles", query, &page); err != nil {
			return nil, err
		}
		for i := range page.Items {
			item := &page.Items[i]
			if item.SourceListingID != nil && *item.SourceListingID == listingID {
				return item, nil
			}
		}
		next := strings.TrimSpace(page.NextPageToken)
		if next == "" {
			return nil, fmt.Errorf("listing %s has no published automatic subscription bundle", listingID)
		}
		if _, exists := seen[next]; exists {
			return nil, fmt.Errorf("subscription bundle pagination returned a repeated token")
		}
		seen[next] = struct{}{}
		pageToken = next
	}
}
