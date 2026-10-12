package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

type skillCategory struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type skillCatalogFee struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
	Unit     string `json:"unit"`
}

type skillCatalogMetadata struct {
	Source        string    `json:"source"`
	ResourceID    string    `json:"resourceId"`
	DisplayName   string    `json:"displayName"`
	Description   string    `json:"description"`
	Category      string    `json:"category"`
	CategoryLabel string    `json:"categoryLabel"`
	IsOfficial    bool      `json:"isOfficial"`
	PinPosition   flexInt64 `json:"pinPosition"`
	Creator       struct {
		Nickname string `json:"nickname"`
	} `json:"creator"`
	SuccessfulTaskCount  flexInt64 `json:"successfulTaskCount"`
	EffectivePublishedAt *string   `json:"effectivePublishedAt"`
}

type skillCatalogItem struct {
	skillCatalogMetadata
	InvocationFee *skillCatalogFee `json:"invocationFee"`
}

type skillCatalogPage struct {
	Items           []skillCatalogItem `json:"items"`
	Page            flexInt64          `json:"page"`
	PageSize        flexInt64          `json:"pageSize"`
	TotalCount      flexInt64          `json:"totalCount"`
	CategoryOptions []skillCategory    `json:"categoryOptions"`
}

type skillCatalogPricing struct {
	CreatorFee               *skillCatalogFee `json:"creatorFee"`
	PayPerUseEnabled         *bool            `json:"payPerUseEnabled"`
	SubscriptionOffersStatus string           `json:"subscriptionOffersStatus"`
	SubscriptionOffers       []struct {
		BundleID   string    `json:"bundleId"`
		BundleName string    `json:"bundleName"`
		Revision   flexInt64 `json:"revision"`
		Tiers      []struct {
			TierType     string         `json:"tierType"`
			Price        *moneyResponse `json:"price"`
			Enabled      bool           `json:"enabled"`
			DurationDays flexInt64      `json:"durationDays"`
		} `json:"tiers"`
	} `json:"subscriptionOffers"`
	SubscriptionPolicy struct {
		PaymentTiming    string `json:"paymentTiming"`
		CreatorFeeWaived *bool  `json:"creatorFeeWaived"`
		ModelFeeIncluded *bool  `json:"modelFeeIncluded"`
	} `json:"subscriptionPolicy"`
	ModelFee struct {
		BillingMode string `json:"billingMode"`
		SettledBy   string `json:"settledBy"`
	} `json:"modelFee"`
}

type skillCatalogDetail struct {
	skillCatalogMetadata
	Version          string              `json:"version"`
	UpdatedAt        string              `json:"updatedAt"`
	ListingVersionID string              `json:"listingVersionId"`
	TemplateID       string              `json:"templateId"`
	Value            marketPublicValue   `json:"value"`
	OutputType       string              `json:"outputType"`
	Pricing          skillCatalogPricing `json:"pricing"`
}

func newMarketListCmd(opts *rootOptions) *cobra.Command {
	var keyword, category, sortBy string
	var page int64
	var pageSize int
	cmd := &cobra.Command{Use: "list", Short: "List official and Market skills with page-number pagination", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			keyword = strings.TrimSpace(keyword)
			if !utf8.ValidString(keyword) || utf8.RuneCountInString(keyword) > 200 || strings.ContainsRune(keyword, 0) {
				return fmt.Errorf("--keyword must contain at most 200 Unicode characters and no NUL")
			}
			if cmd.Flags().Changed("category") && !marketCategoryCode.MatchString(category) {
				return fmt.Errorf("--category must be a category code from market list (categoryOptions)")
			}
			if sortBy != "default" && sortBy != "usage" && sortBy != "newest" {
				return fmt.Errorf("--sort-by must be default, usage or newest")
			}
			if page < 1 || pageSize < 1 || pageSize > 15 {
				return fmt.Errorf("--page must be positive and --page-size must be between 1 and 15")
			}
			if page-1 > math.MaxInt64/int64(pageSize) {
				return fmt.Errorf("--page offset is too large")
			}
			query := url.Values{"page": {strconv.FormatInt(page, 10)}, "pageSize": {strconv.Itoa(pageSize)}, "sortBy": {sortBy}}
			if keyword != "" {
				query.Set("keyword", keyword)
			}
			if category != "" {
				query.Set("category", category)
			}
			raw, err := getSkillCatalog(cmd, opts, "/skillCatalog", query)
			if err != nil {
				return err
			}
			response, err := decodeJSONValue[skillCatalogPage](raw)
			if err != nil {
				return err
			}
			if response.Items == nil || response.CategoryOptions == nil || response.Page < 1 || response.PageSize < 1 || response.PageSize > 15 || response.TotalCount < 0 {
				return fmt.Errorf("invalid skill catalog response: expected items, categoryOptions and page/pageSize/totalCount")
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), raw)
			}
			return printSkillCatalogPage(cmd.OutOrStdout(), response)
		},
	}
	cmd.Flags().StringVar(&keyword, "keyword", "", "Literal search in name, description or category label (at most 200 Unicode characters)")
	cmd.Flags().StringVar(&category, "category", "", "Category code returned by market list (categoryOptions)")
	cmd.Flags().StringVar(&sortBy, "sort-by", "default", "Sort mode: default, usage or newest; pinned skills lead every sort")
	cmd.Flags().Int64Var(&page, "page", 1, "One-based page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 15, "Page size (1-15)")
	return cmd
}

func newMarketShowCmd(opts *rootOptions) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "show <resource-id>", Short: "Show public skill detail, inputs, outputs and subscription prices", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceID := strings.TrimSpace(args[0])
			if (source != "official" && source != "market") || resourceID == "" || strings.ContainsAny(resourceID, "/\x00") {
				return fmt.Errorf("--source must be official or market; resource ID must be a non-empty ID from market list without slash or NUL")
			}
			raw, err := getSkillCatalog(cmd, opts, "/skillCatalog/"+source+"/"+url.PathEscape(resourceID), nil)
			if err != nil {
				return err
			}
			response, err := decodeJSONValue[skillCatalogDetail](raw)
			if err != nil {
				return err
			}
			if response.Source != source || response.ResourceID != resourceID {
				return fmt.Errorf("skill catalog detail identity does not match requested %s/%s", source, resourceID)
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), raw)
			}
			return printSkillCatalogDetail(cmd.OutOrStdout(), response, raw)
		},
	}
	cmd.Flags().StringVar(&source, "source", "market", "Resource source: official or market; use the source returned by market list")
	return cmd
}

func getSkillCatalog(cmd *cobra.Command, opts *rootOptions, path string, query url.Values) (map[string]any, error) {
	httpClient, err := newHTTPClient(opts)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
	defer cancel()
	var body json.RawMessage
	if err := httpClient.GetProductJSONWithQuery(ctx, path, query, &body); err != nil {
		return nil, err
	}
	// Counts and revisions are int64. Preserve them when retaining unknown
	// response fields rather than converting through float64.
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode skill catalog response: %w", err)
	}
	return raw, nil
}

func formatCatalogFee(fee *skillCatalogFee) (string, error) {
	if fee == nil {
		return "", fmt.Errorf("missing catalog fee")
	}
	amount, err := formatResponseMoney(&moneyResponse{Amount: fee.Amount, Currency: fee.Currency}, nil, "")
	if err != nil {
		return "", err
	}
	if fee.Unit != "task" {
		return "", fmt.Errorf("unsupported catalog fee unit %q", fee.Unit)
	}
	return amount + " / task", nil
}

func printSkillCatalogPage(w io.Writer, response skillCatalogPage) error {
	tw := newTabWriter(w)
	if len(response.Items) == 0 {
		if _, err := fmt.Fprintln(tw, "no catalog skills"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(tw, "source\tresource_id\tname\tcategory\tcategory_label\tcreator\tsuccessful_tasks\tpin_position\tinvocation_fee\tpublished_at\tdescription"); err != nil {
			return err
		}
		for _, item := range response.Items {
			fee, err := formatCatalogFee(item.InvocationFee)
			if err != nil {
				return fmt.Errorf("catalog resource %s/%s: %w", item.Source, item.ResourceID, err)
			}
			published := ""
			if item.EffectivePublishedAt != nil {
				published = *item.EffectivePublishedAt
			}
			if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\t%s\n", oneLine(item.Source), oneLine(item.ResourceID), oneLine(item.DisplayName), oneLine(item.Category), oneLine(item.CategoryLabel), oneLine(item.Creator.Nickname), item.SuccessfulTaskCount, item.PinPosition, fee, oneLine(published), oneLine(item.Description)); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(tw, "page\t%d\npage_size\t%d\ntotal_count\t%d\n", response.Page, response.PageSize, response.TotalCount); err != nil {
		return err
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	return printSkillCategories(w, response.CategoryOptions)
}

func printSkillCategories(w io.Writer, categories []skillCategory) error {
	tw := newTabWriter(w)
	if _, err := fmt.Fprintln(tw, "category_code\tlabel"); err != nil {
		return err
	}
	for _, category := range categories {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", oneLine(category.Code), oneLine(category.Label)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func printSkillCatalogDetail(w io.Writer, detail skillCatalogDetail, raw map[string]any) error {
	fee, err := formatCatalogFee(detail.Pricing.CreatorFee)
	if err != nil {
		return fmt.Errorf("catalog creatorFee: %w", err)
	}
	tw := newTabWriter(w)
	for _, row := range [][2]string{
		{"source", detail.Source}, {"resource_id", detail.ResourceID}, {"display_name", detail.DisplayName}, {"description", detail.Description},
		{"category", detail.Category}, {"category_label", detail.CategoryLabel}, {"creator", detail.Creator.Nickname},
		{"is_official", strconv.FormatBool(detail.IsOfficial)}, {"pin_position", strconv.FormatInt(int64(detail.PinPosition), 10)},
		{"successful_tasks", strconv.FormatInt(int64(detail.SuccessfulTaskCount), 10)},
		{"version", detail.Version}, {"updated_at", detail.UpdatedAt}, {"listing_version_id", detail.ListingVersionID}, {"template_id", detail.TemplateID},
		{"creator_fee", fee}, {"output_type", detail.OutputType}, {"subscription_offers_status", detail.Pricing.SubscriptionOffersStatus},
		{"subscription_payment_timing", detail.Pricing.SubscriptionPolicy.PaymentTiming}, {"model_fee_billing_mode", detail.Pricing.ModelFee.BillingMode}, {"model_fee_settled_by", detail.Pricing.ModelFee.SettledBy},
	} {
		if row[1] != "" {
			if _, err := fmt.Fprintf(tw, "%s\t%s\n", row[0], oneLine(row[1])); err != nil {
				return err
			}
		}
	}
	if detail.EffectivePublishedAt != nil {
		if _, err := fmt.Fprintf(tw, "effective_published_at\t%s\n", oneLine(*detail.EffectivePublishedAt)); err != nil {
			return err
		}
	}
	for _, row := range []struct {
		name  string
		value *bool
	}{
		{"pay_per_use_enabled", detail.Pricing.PayPerUseEnabled}, {"subscription_creator_fee_waived", detail.Pricing.SubscriptionPolicy.CreatorFeeWaived}, {"subscription_model_fee_included", detail.Pricing.SubscriptionPolicy.ModelFeeIncluded},
	} {
		value := "unknown"
		if row.value != nil {
			value = strconv.FormatBool(*row.value)
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", row.name, value); err != nil {
			return err
		}
	}
	if err := printMarketValue(tw, detail.Value); err != nil {
		return err
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	for _, offer := range detail.Pricing.SubscriptionOffers {
		for _, tier := range offer.Tiers {
			if tier.Price == nil {
				return fmt.Errorf("subscription offer %s is missing price", offer.BundleID)
			}
			price, err := formatResponseMoney(tier.Price, nil, "")
			if err != nil {
				return fmt.Errorf("subscription offer %s price: %w", offer.BundleID, err)
			}
			if _, err := fmt.Fprintf(w, "subscription_offer\t%s\t%s\trevision=%d\t%s\t%s\tenabled=%t\tduration_days=%d\n", oneLine(offer.BundleID), oneLine(offer.BundleName), offer.Revision, oneLine(tier.TierType), price, tier.Enabled, tier.DurationDays); err != nil {
				return err
			}
		}
	}
	// Preserve the differing official/market public schemas, including field
	// hints, columns, samples, output descriptions and nullable trust counts.
	for _, section := range []string{"input", "columns", "instructions", "sampleRows", "outputs", "trust", "skillPackage"} {
		if err := printPublicDetailSection(w, section, raw[section]); err != nil {
			return err
		}
	}
	return nil
}

func printPublicDetailSection(w io.Writer, name string, value any) error {
	if value == nil {
		return nil
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s:\n%s\n", name, sanitizeRemoteText(string(encoded), true))
	return err
}
