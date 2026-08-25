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
		Short: "LoomLoom model commands",
	}
	cmd.AddCommand(newModelListCmd(opts), newModelTypesCmd(opts))
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
		Short: "List authority-backed models usable in TemplateSpec",
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
			if cmd.Flags().Changed("only-available") && !onlyAvailable {
				return fmt.Errorf("--only-available=false is no longer supported: LoomLoom lists only models with an authoring contract")
			}

			var resp listModelsResponse
			if err := httpClient.GetProductJSONWithQuery(ctx, "/models", query, &resp); err != nil {
				return err
			}
			resp.Models = filterModelsByProvider(resp.Models, provider)
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), resp)
			}
			return printTemplateSpecModels(cmd.OutOrStdout(), resp.Models)
		},
	}
	cmd.Flags().StringVar(&stepType, "step-type", "", "Step type; run `loomloom model types` to discover valid values (required)")
	cmd.Flags().StringVar(&provider, "provider", "", "Client-side provider filter")
	cmd.Flags().BoolVar(&onlyAvailable, "only-available", true, "Deprecated: only authority-backed models are listed")
	return cmd
}

type modelStepTypesResponse struct {
	SchemaVersion string              `json:"schemaVersion"`
	StepTypes     []modelStepTypeItem `json:"stepTypes"`
}

type modelStepTypeItem struct {
	StepType               string   `json:"stepType"`
	Capability             string   `json:"capability"`
	AuthoringModes         []string `json:"authoringModes"`
	AuthoringModelCount    uint32   `json:"authoringModelCount"`
	AuthoringContractCount uint32   `json:"authoringContractCount"`
}

func newModelTypesCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "types",
		Short: "List valid LoomLoom model step types",
		RunE: func(cmd *cobra.Command, _ []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			var resp modelStepTypesResponse
			if err := httpClient.GetProductJSON(ctx, "/modelStepTypes", &resp); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), resp)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			if _, err := fmt.Fprintln(tw, "step_type\tcapability\tauthoring_modes\tmodels\tcontracts"); err != nil {
				return err
			}
			for _, item := range resp.StepTypes {
				if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\n", item.StepType, item.Capability, strings.Join(item.AuthoringModes, ","), item.AuthoringModelCount, item.AuthoringContractCount); err != nil {
					return err
				}
			}
			return tw.Flush()
		},
	}
}
