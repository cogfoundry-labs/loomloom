package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func newModelCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Model catalog commands",
	}
	cmd.AddCommand(newModelListCmd(opts))
	return cmd
}

func newModelListCmd(opts *rootOptions) *cobra.Command {
	var (
		stepType      string
		provider      string
		onlyAvailable bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List executable models",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(stepType) == "" {
				return fmt.Errorf("--step-type is required")
			}
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			query := url.Values{}
			query.Set("stepType", strings.TrimSpace(stepType))
			if strings.TrimSpace(provider) != "" {
				query.Set("provider", strings.TrimSpace(provider))
			}
			if cmd.Flags().Changed("only-available") {
				query.Set("onlyAvailable", fmt.Sprintf("%t", onlyAvailable))
			}

			var resp listModelsResponse
			if err := httpClient.GetProductJSONWithQuery(ctx, "/models", query, &resp); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), resp)
			}
			return printTemplateSpecModels(cmd.OutOrStdout(), resp.Models)
		},
	}
	cmd.Flags().StringVar(&stepType, "step-type", "", "Step type, e.g. text-generate, image-generate, video-generate (required)")
	cmd.Flags().StringVar(&provider, "provider", "", "Provider filter")
	cmd.Flags().BoolVar(&onlyAvailable, "only-available", true, "When false, include unavailable known models")
	return cmd
}
