package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func newCreatorBundleCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "bundle", Short: "Manage creator SkillBot subscription bundles"}
	cmd.AddCommand(
		newCreatorBundleCandidatesCmd(opts),
		newCreatorBundleCreateCmd(opts),
		newCreatorBundleListCmd(opts),
		newCreatorBundleShowCmd(opts),
		newCreatorBundleItemsCmd(opts),
		newCreatorBundleUpdateCmd(opts),
		newCreatorBundleAddItemCmd(opts),
		newCreatorBundleSetTierCmd(opts),
		newCreatorBundleStatusCmd(opts, true),
		newCreatorBundleStatusCmd(opts, false),
	)
	return cmd
}

func newCreatorBundleCandidatesCmd(opts *rootOptions) *cobra.Command {
	var pageSize int
	var pageToken string
	cmd := &cobra.Command{Use: "list-available", Short: "List my SkillBots eligible for bundle membership", RunE: func(cmd *cobra.Command, _ []string) error {
		return subscriptionGetAndPrint(cmd, opts, "/creators/me/subscriptionBundleCandidates", subscriptionPageQuery(pageSize, pageToken))
	}}
	addSubscriptionPageFlags(cmd, &pageSize, &pageToken)
	return cmd
}

func newCreatorBundleCreateCmd(opts *rootOptions) *cobra.Command {
	var name, description, requestID string
	cmd := &cobra.Command{Use: "create", Short: "Create an empty manual subscription bundle", RunE: func(cmd *cobra.Command, _ []string) error {
		requestID = strings.TrimSpace(requestID)
		name = strings.TrimSpace(name)
		if requestID == "" || name == "" {
			return fmt.Errorf("--name and --client-request-id are required")
		}
		return creatorBundleMutation(cmd, opts, "POST", "/creators/me/subscriptionBundles", map[string]any{"clientRequestId": requestID, "name": name, "description": description})
	}}
	cmd.Flags().StringVar(&name, "name", "", "Bundle name")
	cmd.Flags().StringVar(&description, "description", "", "Bundle description")
	cmd.Flags().StringVar(&requestID, "client-request-id", "", "Stable idempotency key for bundle creation")
	return cmd
}

func newCreatorBundleListCmd(opts *rootOptions) *cobra.Command {
	var name, status, pageToken string
	var pageSize int
	cmd := &cobra.Command{Use: "list", Short: "List my subscription bundles", RunE: func(cmd *cobra.Command, _ []string) error {
		query := subscriptionPageQuery(pageSize, pageToken)
		if value := strings.TrimSpace(name); value != "" {
			query.Set("name", value)
		}
		if value := strings.TrimSpace(status); value != "" {
			query.Set("status", value)
		}
		return subscriptionGetAndPrint(cmd, opts, "/creators/me/subscriptionBundles", query)
	}}
	cmd.Flags().StringVar(&name, "name", "", "Filter by bundle name")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: draft|published|unlisted")
	addSubscriptionPageFlags(cmd, &pageSize, &pageToken)
	return cmd
}

func newCreatorBundleShowCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "show <bundle-id>", Short: "Show one creator subscription bundle", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return subscriptionGetAndPrint(cmd, opts, creatorBundlePath(args[0]), nil)
	}}
}

func newCreatorBundleItemsCmd(opts *rootOptions) *cobra.Command {
	var pageSize int
	var pageToken string
	cmd := &cobra.Command{Use: "items <bundle-id>", Short: "List SkillBots in one creator subscription bundle", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return subscriptionGetAndPrint(cmd, opts, creatorBundlePath(args[0])+"/items", subscriptionPageQuery(pageSize, pageToken))
	}}
	addSubscriptionPageFlags(cmd, &pageSize, &pageToken)
	return cmd
}

func newCreatorBundleUpdateCmd(opts *rootOptions) *cobra.Command {
	var name, description string
	var revision int64
	cmd := &cobra.Command{Use: "update <bundle-id>", Short: "Update bundle name and description", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(name) == "" || revision <= 0 {
			return fmt.Errorf("--name and a positive --expected-revision are required")
		}
		return creatorBundleMutation(cmd, opts, "PATCH", creatorBundlePath(args[0]), map[string]any{"name": name, "description": description, "expectedRevision": revision})
	}}
	cmd.Flags().StringVar(&name, "name", "", "New bundle name")
	cmd.Flags().StringVar(&description, "description", "", "New bundle description")
	cmd.Flags().Int64Var(&revision, "expected-revision", 0, "Current bundle revision")
	return cmd
}

func newCreatorBundleAddItemCmd(opts *rootOptions) *cobra.Command {
	var listingIDs []string
	var revision int64
	cmd := &cobra.Command{Use: "add-item <bundle-id>", Short: "Atomically add SkillBots to a manual bundle", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if len(listingIDs) == 0 || revision <= 0 {
			return fmt.Errorf("at least one --listing-id and a positive --expected-revision are required")
		}
		return creatorBundleMutation(cmd, opts, "POST", creatorBundlePath(args[0])+":addItems", map[string]any{"listingIds": listingIDs, "expectedRevision": revision})
	}}
	cmd.Flags().StringSliceVar(&listingIDs, "listing-id", nil, "SkillBot listing ID to add; repeat for multiple IDs")
	cmd.Flags().Int64Var(&revision, "expected-revision", 0, "Current bundle revision")
	return cmd
}

func newCreatorBundleSetTierCmd(opts *rootOptions) *cobra.Command {
	var tier, price, currency string
	var enabled bool
	var revision int64
	cmd := &cobra.Command{Use: "set-tier <bundle-id>", Short: "Create, reprice, enable, or disable a bundle tier", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		tier = strings.TrimSpace(tier)
		price = strings.TrimSpace(price)
		currency = strings.ToUpper(strings.TrimSpace(currency))
		if (tier != "monthly" && tier != "yearly") || price == "" || currency == "" || revision <= 0 {
			return fmt.Errorf("--tier monthly|yearly, --price, --currency, and a positive --expected-revision are required")
		}
		path := creatorBundlePath(args[0]) + "/tiers/" + url.PathEscape(tier)
		payload := map[string]any{"price": map[string]string{"amount": price, "currency": currency}, "enabled": enabled, "expectedRevision": revision}
		return creatorBundleMutation(cmd, opts, "PUT", path, payload)
	}}
	cmd.Flags().StringVar(&tier, "tier", "", "Tier type: monthly|yearly")
	cmd.Flags().StringVar(&price, "price", "", "Decimal tier price")
	cmd.Flags().StringVar(&currency, "currency", "CNY", "Tier currency")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "Whether new purchases may use this tier")
	cmd.Flags().Int64Var(&revision, "expected-revision", 0, "Current bundle revision")
	return cmd
}

func newCreatorBundleStatusCmd(opts *rootOptions, publish bool) *cobra.Command {
	action := "unlist"
	short := "Close bundle sales without affecting existing subscriptions"
	if publish {
		action = "publish"
		short = "Publish or reopen the same subscription bundle"
	}
	var revision int64
	var reason string
	cmd := &cobra.Command{Use: action + " <bundle-id>", Short: short, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if revision <= 0 {
			return fmt.Errorf("--expected-revision must be greater than 0")
		}
		return creatorBundleMutation(cmd, opts, "POST", creatorBundlePath(args[0])+":"+action, map[string]any{"expectedRevision": revision, "reason": reason})
	}}
	cmd.Flags().Int64Var(&revision, "expected-revision", 0, "Current bundle revision")
	if !publish {
		cmd.Flags().StringVar(&reason, "reason", "creator_unlisted", "Audit reason for closing sales")
	}
	return cmd
}

func newListingPaymentSettingsCmd(opts *rootOptions) *cobra.Command {
	var payPerUse, subscription bool
	var revision int64
	cmd := &cobra.Command{Use: "payment-settings <listing-id>", Short: "Update pay-per-use and automatic subscription modes", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !cmd.Flags().Changed("pay-per-use") || !cmd.Flags().Changed("subscription") || revision < 0 {
			return fmt.Errorf("--pay-per-use, --subscription, and --expected-revision must be explicit")
		}
		path := "/creators/me/marketListings/" + url.PathEscape(strings.TrimSpace(args[0])) + "/paymentSettings"
		return creatorBundleMutation(cmd, opts, "PUT", path, map[string]any{"payPerUse": payPerUse, "subscription": subscription, "expectedRevision": revision})
	}}
	cmd.Flags().BoolVar(&payPerUse, "pay-per-use", true, "Enable per-run creator pricing")
	cmd.Flags().BoolVar(&subscription, "subscription", false, "Enable the stable automatic single-SkillBot bundle")
	cmd.Flags().Int64Var(&revision, "expected-revision", -1, "Current payment settings revision; use 0 for first creation")
	return cmd
}

func creatorBundlePath(raw string) string {
	return "/creators/me/subscriptionBundles/" + url.PathEscape(strings.TrimSpace(raw))
}

func creatorBundleMutation(cmd *cobra.Command, opts *rootOptions, method, path string, payload any) error {
	httpClient, err := newHTTPClient(opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
	defer cancel()
	var response map[string]any
	switch method {
	case "POST":
		err = httpClient.PostProductJSON(ctx, path, payload, &response)
	case "PUT":
		err = httpClient.PutProductJSON(ctx, path, payload, &response)
	case "PATCH":
		err = httpClient.PatchProductJSON(ctx, path, payload, &response)
	default:
		err = fmt.Errorf("unsupported creator bundle method %s", method)
	}
	if err != nil {
		return err
	}
	return writeIndentedJSON(cmd.OutOrStdout(), response)
}
