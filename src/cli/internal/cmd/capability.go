package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

type authoringCapabilityResolution struct {
	SchemaVersion string                     `json:"schemaVersion"`
	Status        string                     `json:"status"`
	Query         authoringCapabilityQuery   `json:"query"`
	Matches       []authoringCapabilityMatch `json:"matches"`
	NextAction    string                     `json:"nextAction"`
}

type authoringCapabilityQuery struct {
	InputModalities []string `json:"inputModalities"`
	OutputModality  string   `json:"outputModality"`
	ModelID         string   `json:"modelId,omitempty"`
}

type authoringCapabilityMatch struct {
	AuthoringKind           string                     `json:"authoringKind"`
	Operation               string                     `json:"operation"`
	StepType                string                     `json:"stepType"`
	InputModalities         []string                   `json:"inputModalities"`
	RequiredInputModalities []string                   `json:"requiredInputModalities"`
	OutputModalities        []string                   `json:"outputModalities"`
	Profile                 *templateAuthoringProfile  `json:"profile,omitempty"`
	Contract                *templateAuthoringContract `json:"contract,omitempty"`
	EligibleModels          []modelSummary             `json:"eligibleModels"`
}

func newCapabilityCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "capability", Short: "Resolve LoomLoom capabilities from task intent"}
	cmd.AddCommand(newCapabilityResolveCmd(opts))
	return cmd
}

func newCapabilityResolveCmd(opts *rootOptions) *cobra.Command {
	var inputModalities []string
	var outputModality string
	var modelID string
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve modalities to current TemplateSpec authoring choices",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if len(inputModalities) == 0 {
				return fmt.Errorf("at least one --input is required")
			}
			if strings.TrimSpace(outputModality) == "" {
				return fmt.Errorf("--output-modality is required")
			}
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			query := url.Values{}
			for _, modality := range inputModalities {
				for _, value := range strings.Split(modality, ",") {
					if normalized := strings.TrimSpace(value); normalized != "" {
						query.Add("inputModality", normalized)
					}
				}
			}
			query.Set("outputModality", strings.TrimSpace(outputModality))
			if normalized := strings.TrimSpace(modelID); normalized != "" {
				query.Set("modelId", normalized)
			}
			var resp authoringCapabilityResolution
			if err := httpClient.GetProductJSONWithQuery(ctx, "/authoringCapabilities:resolve", query, &resp); err != nil {
				return err
			}
			if opts.output == "json" {
				return writeIndentedJSON(cmd.OutOrStdout(), resp)
			}
			return printAuthoringCapabilityResolution(cmd, resp)
		},
	}
	cmd.Flags().StringSliceVar(&inputModalities, "input", nil, "Required input modality; repeat for multiple values")
	cmd.Flags().StringVar(&outputModality, "output-modality", "", "Required output modality")
	cmd.Flags().StringVar(&modelID, "model-id", "", "Optional exact model ID")
	return cmd
}

func printAuthoringCapabilityResolution(cmd *cobra.Command, resp authoringCapabilityResolution) error {
	if len(resp.Matches) == 0 {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "status: %s\nnext_action: %s\n", resp.Status, resp.NextAction)
		return err
	}
	tw := newTabWriter(cmd.OutOrStdout())
	if _, err := fmt.Fprintln(tw, "operation\tauthoring_kind\tstep_type\tauthority\tmodels"); err != nil {
		return err
	}
	for _, match := range resp.Matches {
		authority := ""
		if match.Profile != nil {
			authority = match.Profile.ProfileID + "@" + match.Profile.Revision
		}
		if match.Contract != nil {
			authority = match.Contract.SubjectRevisionID
		}
		models := make([]string, 0, len(match.EligibleModels))
		for _, model := range match.EligibleModels {
			models = append(models, model.ModelID)
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", match.Operation, match.AuthoringKind, match.StepType, authority, strings.Join(models, ",")); err != nil {
			return err
		}
	}
	return tw.Flush()
}
