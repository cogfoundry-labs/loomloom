package cmd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// creatorMarketListingResponse mirrors the backend marketListingResponse
// returned by /creators/me/marketListings endpoints.
type creatorMarketListingResponse struct {
	ID                          string    `json:"id"`
	DisplayName                 string    `json:"displayName"`
	Description                 string    `json:"description"`
	Status                      string    `json:"status"`
	PublishedVersionID          string    `json:"publishedVersionId"`
	ListingVersionID            string    `json:"listingVersionId"`
	ReviewStatus                string    `json:"reviewStatus"`
	ReviewReason                string    `json:"reviewReason"`
	TaskFixedFeeT               flexInt64 `json:"taskFixedFeeT"`
	Currency                    string    `json:"currency"`
	SaleStatus                  string    `json:"saleStatus"`
	ExecutionAvailabilityStatus string    `json:"executionAvailabilityStatus"`
}

type creatorMarketListingsResponse struct {
	Items         []creatorMarketListingResponse `json:"items"`
	NextPageToken string                         `json:"nextPageToken,omitempty"`
}

type creatorMarketListingVersionResponse struct {
	ID                          string    `json:"id"`
	ListingID                   string    `json:"listingId"`
	VersionNumber               flexInt64 `json:"versionNumber"`
	Status                      string    `json:"status"`
	SaleStatus                  string    `json:"saleStatus"`
	ExecutionAvailabilityStatus string    `json:"executionAvailabilityStatus"`
	ExecutionBlockReason        string    `json:"executionBlockReason"`
	ReviewStatus                string    `json:"reviewStatus"`
	ReviewReason                string    `json:"reviewReason"`
	TaskFixedFeeT               flexInt64 `json:"taskFixedFeeT"`
	Currency                    string    `json:"currency"`
	CreatedAtUnix               flexInt64 `json:"createdAtUnix"`
}

// creatorMarketListingVersionsResponse mirrors the backend
// marketListingVersionsResponse, which is not paginated beyond its pageSize
// limit and returns no continuation token.
type creatorMarketListingVersionsResponse struct {
	Items []creatorMarketListingVersionResponse `json:"items"`
}

func newListingCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listing",
		Short: "Creator listing commands",
	}
	cmd.AddCommand(
		newListingPublishCmd(opts),
		newListingUpdateCmd(opts),
		newListingListCmd(opts),
		newListingShowCmd(opts),
		newListingVersionsCmd(opts),
		newListingWithdrawCmd(opts),
		newListingUnlistCmd(opts),
		newListingRelistCmd(opts),
	)
	return cmd
}

func newListingPublishCmd(opts *rootOptions) *cobra.Command {
	var (
		listingID         string
		templateVersionID string
		displayName       string
		description       string
		taskFixedFeeT     int64
	)
	cmd := &cobra.Command{
		Use:   "publish <template-id>",
		Short: "Submit a template version for Market SkillBot review",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			req := publishMarketListingRequest{
				ListingID:         strings.TrimSpace(listingID),
				TemplateID:        strings.TrimSpace(args[0]),
				TemplateVersionID: strings.TrimSpace(templateVersionID),
				DisplayName:       strings.TrimSpace(displayName),
				Description:       strings.TrimSpace(description),
				TaskFixedFeeT:     taskFixedFeeT,
			}

			var resp map[string]any
			if err := httpClient.PostProductJSON(ctx, "/marketListings", req, &resp); err != nil {
				if strings.Contains(err.Error(), "must have a successful run") {
					return fmt.Errorf("%w\nHint: submit at least one successful run (template-spec submit-workbook) before publishing", err)
				}
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&listingID, "listing-id", "", "Existing listing ID when publishing a new version")
	cmd.Flags().StringVar(&templateVersionID, "template-version-id", "", "Template version ID to publish")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Market SkillBot display name")
	cmd.Flags().StringVar(&description, "description", "", "Market SkillBot description")
	cmd.Flags().Int64Var(&taskFixedFeeT, "task-fixed-fee-t", 0, "Creator fixed fee per billable task, in API units")
	_ = cmd.MarkFlagRequired("template-version-id")
	_ = cmd.MarkFlagRequired("display-name")
	_ = cmd.MarkFlagRequired("task-fixed-fee-t")
	return cmd
}

func newListingListCmd(opts *rootOptions) *cobra.Command {
	var (
		keyword   string
		pageSize  int
		pageToken string
		orderBy   string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List my Market SkillBot listings",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			}
			if strings.TrimSpace(pageToken) != "" {
				query.Set("pageToken", strings.TrimSpace(pageToken))
			}
			if strings.TrimSpace(orderBy) != "" {
				query.Set("orderBy", strings.TrimSpace(orderBy))
			}

			var raw map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/creators/me/marketListings", query, &raw); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), raw)
			}
			resp, err := decodeJSONValue[creatorMarketListingsResponse](raw)
			if err != nil {
				return err
			}
			return printCreatorListings(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&keyword, "keyword", "", "Search keyword")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Page size")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "Pagination token from previous response")
	cmd.Flags().StringVar(&orderBy, "order-by", "", "Sort expression, e.g. 'createdAt desc'")
	return cmd
}

func newListingShowCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show <listing-id>",
		Short: "Show one creator-owned Market SkillBot listing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			path := "/creators/me/marketListings/" + url.PathEscape(strings.TrimSpace(args[0]))
			var raw map[string]any
			if err := httpClient.GetProductJSON(ctx, path, &raw); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), raw)
			}
			resp, err := decodeJSONValue[creatorMarketListingResponse](raw)
			if err != nil {
				return err
			}
			return printCreatorListingDetail(cmd.OutOrStdout(), resp)
		},
	}
}

func newListingVersionsCmd(opts *rootOptions) *cobra.Command {
	var pageSize int
	cmd := &cobra.Command{
		Use:   "versions <listing-id>",
		Short: "List versions of a creator-owned Market SkillBot listing",
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

			path := "/creators/me/marketListings/" + url.PathEscape(strings.TrimSpace(args[0])) + "/versions"
			var raw map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, path, query, &raw); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), raw)
			}
			resp, err := decodeJSONValue[creatorMarketListingVersionsResponse](raw)
			if err != nil {
				return err
			}
			return printCreatorListingVersions(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Page size")
	return cmd
}

func newListingUpdateCmd(opts *rootOptions) *cobra.Command {
	var (
		displayName string
		description string
	)
	cmd := &cobra.Command{
		Use:   "update <listing-id>",
		Short: "Submit a public profile update for review (display name / description)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("display-name") && !cmd.Flags().Changed("description") {
				return fmt.Errorf("at least one of --display-name or --description is required")
			}
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			listingID := strings.TrimSpace(args[0])
			current := creatorListingProfile{}
			if !cmd.Flags().Changed("display-name") || !cmd.Flags().Changed("description") {
				path := "/creators/me/marketListings/" + url.PathEscape(listingID)
				if err := httpClient.GetProductJSON(ctx, path, &current); err != nil {
					return fmt.Errorf("get current listing profile: %w", err)
				}
			}

			nextDisplayName := strings.TrimSpace(displayName)
			if !cmd.Flags().Changed("display-name") {
				nextDisplayName = strings.TrimSpace(current.DisplayName)
			}
			if nextDisplayName == "" {
				return fmt.Errorf("display name is required; pass --display-name or ensure the current listing has one")
			}

			nextDescription := strings.TrimSpace(description)
			if !cmd.Flags().Changed("description") {
				nextDescription = strings.TrimSpace(current.Description)
			}

			payload := map[string]any{
				"displayName": nextDisplayName,
				"description": nextDescription,
			}

			path := "/marketListings/" + url.PathEscape(listingID) + ":updatePublicProfile"
			var resp map[string]any
			if err := httpClient.PostProductJSON(ctx, path, payload, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&displayName, "display-name", "", "New display name")
	cmd.Flags().StringVar(&description, "description", "", "New description")
	return cmd
}

type creatorListingProfile struct {
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

type creatorReviewSummary struct {
	ID        string `json:"id"`
	ListingID string `json:"listingId"`
	Status    string `json:"status"`
}

type creatorReviewListResponse struct {
	Items []creatorReviewSummary `json:"items"`
}

func newListingWithdrawCmd(opts *rootOptions) *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "withdraw <listing-id>",
		Short: "Withdraw the pending review request for a listing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			query := url.Values{}
			query.Set("status", "pending")
			query.Set("pageSize", "500")
			opts.debugf("listing withdraw: resolving pending review listing_id=%s", strings.TrimSpace(args[0]))
			var reviews creatorReviewListResponse
			if err := httpClient.GetProductJSONWithQuery(ctx, "/creators/me/marketReviewRequests", query, &reviews); err != nil {
				return fmt.Errorf("list pending review requests: %w", err)
			}

			listingID := strings.TrimSpace(args[0])
			reviewIDs := make([]string, 0, 1)
			for _, review := range reviews.Items {
				if strings.TrimSpace(review.ListingID) == listingID &&
					strings.EqualFold(strings.TrimSpace(review.Status), "pending") &&
					strings.TrimSpace(review.ID) != "" {
					reviewIDs = append(reviewIDs, strings.TrimSpace(review.ID))
				}
			}
			sort.Strings(reviewIDs)
			switch len(reviewIDs) {
			case 0:
				return fmt.Errorf("listing %s has no pending review request", listingID)
			case 1:
			default:
				return fmt.Errorf(
					"listing %s has multiple pending review requests: %s; withdraw one explicitly with creator review withdraw <review-request-id>",
					listingID,
					strings.Join(reviewIDs, ", "),
				)
			}

			opts.debugf("listing withdraw: pending review resolved review_request_id=%s", reviewIDs[0])
			payload := map[string]any{}
			if strings.TrimSpace(reason) != "" {
				payload["reason"] = strings.TrimSpace(reason)
			}
			path := "/creators/me/marketReviewRequests/" + url.PathEscape(reviewIDs[0]) + ":withdraw"
			var resp map[string]any
			if err := httpClient.PostProductJSON(ctx, path, payload, &resp); err != nil {
				return fmt.Errorf("withdraw pending review request %s: %w", reviewIDs[0], err)
			}
			opts.debugf("listing withdraw: review withdrawn review_request_id=%s", reviewIDs[0])
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Optional withdrawal reason")
	return cmd
}

func newListingUnlistCmd(opts *rootOptions) *cobra.Command {
	return newMarketSaleStatusCmd(opts, "unlist", "Unlist a Market SkillBot listing", "unlist")
}

func newListingRelistCmd(opts *rootOptions) *cobra.Command {
	return newMarketSaleStatusCmd(opts, "relist", "Restore a previously unlisted Market SkillBot listing", "list")
}

func printCreatorListings(w io.Writer, resp creatorMarketListingsResponse) error {
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
	if _, err := fmt.Fprintln(tw, "id\tname\ttask_fixed_fee\ttask_fixed_fee_t\tstatus\tsale_status\texecution_availability_status\treview_status\tversion"); err != nil {
		return err
	}
	for _, item := range resp.Items {
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			oneLine(item.DisplayName),
			formatMoneyT(int64(item.TaskFixedFeeT), item.Currency),
			int64(item.TaskFixedFeeT),
			oneLine(item.Status),
			oneLine(item.SaleStatus),
			oneLine(item.ExecutionAvailabilityStatus),
			oneLine(item.ReviewStatus),
			oneLine(item.ListingVersionID),
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

func printCreatorListingDetail(w io.Writer, listing creatorMarketListingResponse) error {
	tw := newTabWriter(w)
	for _, row := range [][2]string{
		{"id", listing.ID},
		{"display_name", listing.DisplayName},
		{"description", listing.Description},
		{"status", listing.Status},
		{"published_version_id", listing.PublishedVersionID},
		{"listing_version_id", listing.ListingVersionID},
		{"review_status", listing.ReviewStatus},
		{"review_reason", listing.ReviewReason},
		{"task_fixed_fee", formatMoneyT(int64(listing.TaskFixedFeeT), listing.Currency)},
		{"task_fixed_fee_t", fmt.Sprintf("%d", int64(listing.TaskFixedFeeT))},
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
	return tw.Flush()
}

func printCreatorListingVersions(w io.Writer, resp creatorMarketListingVersionsResponse) error {
	if len(resp.Items) == 0 {
		_, err := fmt.Fprintln(w, "no listing versions")
		return err
	}
	tw := newTabWriter(w)
	if _, err := fmt.Fprintln(tw, "id\tversion\ttask_fixed_fee\ttask_fixed_fee_t\tstatus\tsale_status\texecution_availability_status\treview_status\tcreated_at"); err != nil {
		return err
	}
	type versionNote struct {
		id     string
		label  string
		reason string
	}
	notes := make([]versionNote, 0)
	for _, item := range resp.Items {
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%d\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			int64(item.VersionNumber),
			formatMoneyT(int64(item.TaskFixedFeeT), item.Currency),
			int64(item.TaskFixedFeeT),
			oneLine(item.Status),
			oneLine(item.SaleStatus),
			oneLine(item.ExecutionAvailabilityStatus),
			oneLine(item.ReviewStatus),
			formatUnix(int64(item.CreatedAtUnix)),
		); err != nil {
			return err
		}
		if strings.TrimSpace(item.ExecutionBlockReason) != "" {
			notes = append(notes, versionNote{id: item.ID, label: "execution_block_reason", reason: item.ExecutionBlockReason})
		}
		if strings.TrimSpace(item.ReviewReason) != "" {
			notes = append(notes, versionNote{id: item.ID, label: "review_reason", reason: item.ReviewReason})
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(notes) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "notes:"); err != nil {
		return err
	}
	for _, note := range notes {
		if _, err := fmt.Fprintf(w, "- %s %s: %s\n", note.id, note.label, oneLine(note.reason)); err != nil {
			return err
		}
	}
	return nil
}
