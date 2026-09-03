package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/client"
	templatespecdocs "github.com/cogfoundry-labs/loomloom/src/cli/internal/template_spec_docs"
	"github.com/spf13/cobra"
)

type createUserTemplateResponse struct {
	TemplateID  string    `json:"templateId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   flexInt64 `json:"createdAt"`
}

type saveTemplateVersionResponse struct {
	VersionID      string    `json:"versionId"`
	VersionNumber  flexInt64 `json:"versionNumber"`
	DefinitionHash string    `json:"definitionHash"`
	CreatedAt      flexInt64 `json:"createdAt"`
}

type validateTemplateSpecResponse struct {
	Valid                bool   `json:"valid"`
	PrimaryOutputType    string `json:"primaryOutputType"`
	DefinitionHash       string `json:"definitionHash"`
	ContractBundleHash   string `json:"contractBundleHash"`
	AuthorityFingerprint string `json:"authorityFingerprint"`
}

type templateVersionSpecResponse struct {
	TemplateID     string          `json:"templateId"`
	VersionID      string          `json:"versionId"`
	VersionNumber  flexInt64       `json:"versionNumber"`
	SpecVersion    string          `json:"specVersion"`
	CanonicalSpec  json.RawMessage `json:"canonicalSpec"`
	DefinitionHash string          `json:"definitionHash"`
	VersionNote    string          `json:"versionNote,omitempty"`
	CreatedBy      flexInt64       `json:"createdBy,omitempty"`
	CreatedAtUnix  flexInt64       `json:"createdAtUnix"`
}

type submitUserTemplateWorkbookResponse struct {
	RunID      string    `json:"runId"`
	Status     string    `json:"status"`
	AcceptedAt flexInt64 `json:"acceptedAt"`
}

func (r *submitUserTemplateWorkbookResponse) UnmarshalJSON(data []byte) error {
	type alias struct {
		RunID             string    `json:"runId"`
		Status            string    `json:"status"`
		AcceptedAt        flexInt64 `json:"acceptedAt"`
		AcceptedAtUnix    flexInt64 `json:"acceptedAtUnix"`
		AcceptedAtUnixAlt flexInt64 `json:"accepted_at_unix"`
	}
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	r.RunID = parsed.RunID
	r.Status = parsed.Status
	r.AcceptedAt = parsed.AcceptedAt
	if r.AcceptedAt == 0 {
		r.AcceptedAt = parsed.AcceptedAtUnix
	}
	if r.AcceptedAt == 0 {
		r.AcceptedAt = parsed.AcceptedAtUnixAlt
	}
	return nil
}

type listModelsResponse struct {
	Models []modelSummary `json:"models"`
}

type modelSummary struct {
	ModelID            string                 `json:"modelId"`
	DisplayName        string                 `json:"displayName"`
	SupportedStepTypes []string               `json:"supportedStepTypes"`
	AuthoringOptions   []modelAuthoringOption `json:"authoringOptions"`
}

type modelAuthoringOption struct {
	Kind               string                        `json:"kind"`
	FixedModelContract map[string]any                `json:"fixedModelContract,omitempty"`
	CapabilityProfile  *modelCapabilityProfileOption `json:"capabilityProfile,omitempty"`
}

type modelCapabilityProfileOption struct {
	ProfileID       string                         `json:"profileId"`
	ProfileRevision string                         `json:"profileRevision"`
	ProfileHash     string                         `json:"profileHash"`
	IsDefault       bool                           `json:"isDefault"`
	InputPorts      []templateAuthoringProfilePort `json:"inputPorts"`
	Dynamic         bool                           `json:"dynamic,omitempty"`
}

type templateAuthoringContractsResponse struct {
	Contracts []templateAuthoringContract `json:"contracts"`
}

type templateAuthoringContextResponse struct {
	Profiles []templateAuthoringProfile `json:"profiles"`
}

type templateAuthoringProfile struct {
	ProfileID             string                              `json:"profileId"`
	Revision              string                              `json:"revision"`
	CanonicalHash         string                              `json:"canonicalHash"`
	Capability            string                              `json:"capability"`
	Endpoint              string                              `json:"endpoint"`
	Compiler              string                              `json:"compiler"`
	Stream                bool                                `json:"stream"`
	InputPorts            []templateAuthoringProfilePort      `json:"inputPorts"`
	Output                templateAuthoringProfileOutput      `json:"output"`
	EligibleModels        []modelSummary                      `json:"eligibleModels"`
	Dynamic               *bool                               `json:"dynamic,omitempty"`
	Definition            *dynamicCapabilityProfileDefinition `json:"definition,omitempty"`
	Operations            *dynamicCapabilityProfileOperations `json:"operations,omitempty"`
	DefaultModelAvailable *bool                               `json:"defaultModelAvailable,omitempty"`
}

type templateAuthoringProfileOutput struct {
	Text  bool `json:"text"`
	Usage bool `json:"usage"`
}

type dynamicCapabilityProfileDefinition struct {
	SchemaVersion string                         `json:"schemaVersion"`
	ExecutionUnit string                         `json:"executionUnit"`
	Operation     string                         `json:"operation"`
	Inputs        []templateAuthoringProfilePort `json:"inputs"`
	Outputs       []templateAuthoringProfilePort `json:"outputs"`
	Constraints   map[string]any                 `json:"constraints"`
}

type dynamicCapabilityProfileOperations struct {
	DisplayName    string `json:"displayName"`
	Description    string `json:"description"`
	DefaultModelID string `json:"defaultModelId"`
	SortOrder      int32  `json:"sortOrder"`
	Recommended    bool   `json:"recommended"`
}

type templateAuthoringProfilePort struct {
	PortID            string   `json:"portId"`
	Kind              string   `json:"kind"`
	ValueType         string   `json:"valueType,omitempty"`
	Required          bool     `json:"required"`
	AcceptedMIMETypes []string `json:"acceptedMimeTypes,omitempty"`
	MinItems          int      `json:"minItems,omitempty"`
	MaxItems          int      `json:"maxItems,omitempty"`
}

type templateAuthoringContract struct {
	SubjectRevisionID string                        `json:"subjectRevisionId"`
	SubjectHash       string                        `json:"subjectHash"`
	ModelID           string                        `json:"modelId"`
	Operation         string                        `json:"operation"`
	Variant           string                        `json:"variant"`
	ExecutionUnitRef  string                        `json:"executionUnitRef"`
	InputPorts        []templateAuthoringInputPort  `json:"inputPorts"`
	OutputPorts       []templateAuthoringOutputPort `json:"outputPorts"`
}

type templateAuthoringInputPort struct {
	PortID            string          `json:"portId"`
	Kind              string          `json:"kind"`
	ValueType         string          `json:"valueType,omitempty"`
	Required          bool            `json:"required"`
	Constraints       json.RawMessage `json:"constraints,omitempty"`
	MinItems          int32           `json:"minItems,omitempty"`
	MaxItems          int32           `json:"maxItems,omitempty"`
	AcceptedMIMETypes []string        `json:"acceptedMimeTypes,omitempty"`
	Sequence          json.RawMessage `json:"sequence,omitempty"`
	Label             string          `json:"label,omitempty"`
	Description       string          `json:"description,omitempty"`
}

type templateAuthoringOutputPort struct {
	PortID string `json:"portId"`
	Type   string `json:"type"`
}

func newTemplateSpecCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template-spec",
		Short: "Author user templates from TemplateSpec JSON",
	}
	cmd.AddCommand(
		newTemplateSpecCheckCmd(opts),
		newTemplateSpecDocsCmd(opts),
		newTemplateSpecModelsCmd(opts),
		newTemplateSpecAuthoringContextCmd(opts),
		newTemplateSpecContractsCmd(opts),
		newTemplateSpecListCmd(opts),
		newTemplateSpecGetCmd(opts),
		newTemplateSpecVersionsCmd(opts),
		newTemplateSpecGetVersionCmd(opts),
		newTemplateSpecCreateCmd(opts),
		newTemplateSpecCreateVersionCmd(opts),
		newTemplateSpecDownloadWorkbookCmd(opts),
		newTemplateSpecValidateWorkbookCmd(opts),
		newTemplateSpecPrecheckWorkbookCmd(opts),
		newTemplateSpecSubmitWorkbookCmd(opts),
		newTemplateSpecEstimateCmd(opts),
		newTemplateSpecPrecheckCmd(opts),
		newTemplateSpecRunCmd(opts),
	)
	return cmd
}

func newTemplateSpecAuthoringContextCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "authoring-context",
		Short: "Show the current server-side Profile contracts and eligible models for TemplateSpec v2",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			var resp templateAuthoringContextResponse
			if err := httpClient.GetJSON(ctx, "/templateAuthoringContext", &resp); err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}
			if len(resp.Profiles) == 0 {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "no authoring profiles")
				return err
			}
			tw := newTabWriter(cmd.OutOrStdout())
			if _, err := fmt.Fprintln(tw, "profile_id\trevision\tcapability\tendpoint\tdynamic\toperation\tdefault_model\tdefault_available\tinputs\toutputs\teligible_models"); err != nil {
				return err
			}
			for _, profile := range resp.Profiles {
				modelIDs := make([]string, 0, len(profile.EligibleModels))
				for _, candidate := range profile.EligibleModels {
					modelIDs = append(modelIDs, candidate.ModelID)
				}
				dynamic, defaultAvailable := "-", "-"
				if profile.Dynamic != nil {
					dynamic = fmt.Sprint(*profile.Dynamic)
				}
				if profile.DefaultModelAvailable != nil {
					defaultAvailable = fmt.Sprint(*profile.DefaultModelAvailable)
				}
				operation, defaultModel, inputs, outputs := "", "", formatTemplateAuthoringPorts(profile.InputPorts), ""
				if profile.Definition != nil {
					operation = profile.Definition.Operation
					inputs = formatTemplateAuthoringPorts(profile.Definition.Inputs)
					outputs = formatTemplateAuthoringPorts(profile.Definition.Outputs)
				}
				if profile.Operations != nil {
					defaultModel = profile.Operations.DefaultModelID
				}
				if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", profile.ProfileID, profile.Revision, profile.Capability, profile.Endpoint, dynamic, operation, defaultModel, defaultAvailable, inputs, outputs, strings.Join(modelIDs, ",")); err != nil {
					return err
				}
			}
			return tw.Flush()
		},
	}
}

func formatTemplateAuthoringPorts(ports []templateAuthoringProfilePort) string {
	formatted := make([]string, 0, len(ports))
	for _, port := range ports {
		typeName := port.ValueType
		if typeName == "" {
			typeName = port.Kind
		}
		if len(port.AcceptedMIMETypes) > 0 {
			typeName += "(" + strings.Join(port.AcceptedMIMETypes, ",") + ")"
		}
		if port.Required {
			typeName += "!"
		}
		formatted = append(formatted, port.PortID+":"+typeName)
	}
	return strings.Join(formatted, ",")
}

func newTemplateSpecContractsCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "contracts <model-id>",
		Short: "List enabled model contracts that can be referenced by TemplateSpec v2",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelID := strings.TrimSpace(args[0])
			if modelID == "" {
				return errors.New("model-id is required")
			}
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			query := url.Values{}
			query.Set("modelId", modelID)
			var resp templateAuthoringContractsResponse
			if err := httpClient.GetJSONWithQuery(ctx, "/modelContracts", query, &resp); err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}
			if len(resp.Contracts) == 0 {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "no enabled authoring contracts")
				return err
			}
			tw := newTabWriter(cmd.OutOrStdout())
			_, _ = fmt.Fprintln(tw, "operation\tvariant\tsubject_revision_id\texecution_unit\tinput_ports")
			for _, contract := range resp.Contracts {
				ports := make([]string, 0, len(contract.InputPorts))
				for _, port := range contract.InputPorts {
					ports = append(ports, port.PortID)
				}
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", contract.Operation, contract.Variant, contract.SubjectRevisionID, contract.ExecutionUnitRef, strings.Join(ports, ","))
			}
			return tw.Flush()
		},
	}
}

func newTemplateSpecDocsCmd(opts *rootOptions) *cobra.Command {
	var language string
	cmd := &cobra.Command{
		Use:   "docs [topic]",
		Short: "Show the TemplateSpec documentation snapshot shipped with this CLI",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedLanguage, err := templatespecdocs.NormalizeLanguage(strings.TrimSpace(language))
			if err != nil {
				return err
			}
			topic := ""
			if len(args) > 0 {
				topic = strings.TrimSpace(args[0])
			}
			if topic == "" {
				return printTemplateSpecDocsIndex(cmd, opts, normalizedLanguage)
			}
			if topic == "all" {
				return printAllTemplateSpecDocs(cmd, opts, normalizedLanguage)
			}
			return printOneTemplateSpecDoc(cmd, opts, normalizedLanguage, topic)
		},
	}
	cmd.Flags().StringVar(&language, "lang", "en", "Documentation language: en or zh-CN")
	return cmd
}

func printTemplateSpecDocsIndex(cmd *cobra.Command, opts *rootOptions, language string) error {
	topics, err := templatespecdocs.Topics(language)
	if err != nil {
		return err
	}
	manifest, err := templatespecdocs.ReadManifest()
	if err != nil {
		return err
	}
	if opts.output == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"language":          language,
			"languageRevision":  templateSpecLanguageRevision(manifest, language),
			"specRevision":      manifest.SpecRevision,
			"generatedRevision": manifest.GeneratedRevision,
			"owner":             manifest.Owner,
			"topics":            topics,
			"usage":             "loomloom template-spec docs <topic>",
		})
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "TemplateSpec revision: %s\nLanguage: %s\nLanguage revision: %s\nGenerated revision: %s\nOwner: %s\n\nTemplateSpec docs topics:\n", manifest.SpecRevision, language, templateSpecLanguageRevision(manifest, language), manifest.GeneratedRevision, manifest.Owner)
	if err != nil {
		return err
	}
	tw := newTabWriter(cmd.OutOrStdout())
	if _, err := fmt.Fprintln(tw, "topic\tdescription"); err != nil {
		return err
	}
	for _, topic := range topics {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", topic.Name, topic.Description); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "\nUse: loomloom template-spec docs <topic>")
	return err
}

func printOneTemplateSpecDoc(cmd *cobra.Command, opts *rootOptions, language string, topicName string) error {
	topic, content, err := templatespecdocs.Read(language, topicName)
	if err != nil {
		return err
	}
	manifest, err := templatespecdocs.ReadManifest()
	if err != nil {
		return err
	}
	if opts.output == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"language":          language,
			"languageRevision":  templateSpecLanguageRevision(manifest, language),
			"specRevision":      manifest.SpecRevision,
			"generatedRevision": manifest.GeneratedRevision,
			"owner":             manifest.Owner,
			"topic":             topic.Name,
			"filename":          topic.Filename,
			"description":       topic.Description,
			"content":           content,
		})
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "TemplateSpec revision: %s\nLanguage: %s\nLanguage revision: %s\nGenerated revision: %s\nOwner: %s\n\n%s", manifest.SpecRevision, language, templateSpecLanguageRevision(manifest, language), manifest.GeneratedRevision, manifest.Owner, content)
	return err
}

func printAllTemplateSpecDocs(cmd *cobra.Command, opts *rootOptions, language string) error {
	topics, err := templatespecdocs.Topics(language)
	if err != nil {
		return err
	}
	manifest, err := templatespecdocs.ReadManifest()
	if err != nil {
		return err
	}
	docs := make([]map[string]any, 0, len(topics))
	for _, topic := range topics {
		loadedTopic, content, err := templatespecdocs.Read(language, topic.Name)
		if err != nil {
			return err
		}
		docs = append(docs, map[string]any{
			"topic":       loadedTopic.Name,
			"filename":    loadedTopic.Filename,
			"description": loadedTopic.Description,
			"content":     content,
		})
	}
	if opts.output == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"language": language, "languageRevision": templateSpecLanguageRevision(manifest, language), "specRevision": manifest.SpecRevision, "generatedRevision": manifest.GeneratedRevision, "owner": manifest.Owner, "docs": docs})
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "TemplateSpec revision: %s\nLanguage: %s\nLanguage revision: %s\nGenerated revision: %s\nOwner: %s\n\n", manifest.SpecRevision, language, templateSpecLanguageRevision(manifest, language), manifest.GeneratedRevision, manifest.Owner); err != nil {
		return err
	}
	for i, doc := range docs {
		if i > 0 {
			if _, err := fmt.Fprint(cmd.OutOrStdout(), "\n---\n\n"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(cmd.OutOrStdout(), doc["content"]); err != nil {
			return err
		}
	}
	return nil
}

func templateSpecLanguageRevision(manifest templatespecdocs.Manifest, language string) string {
	if language == "zh-CN" {
		return manifest.ChineseRevision
	}
	return manifest.EnglishRevision
}

func newTemplateSpecCheckCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "check <spec-json>",
		Short: "Validate a TemplateSpec v2 file against the current server authority",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := loadTemplateSpecTransportFile(args[0])
			if err != nil {
				return err
			}
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			result, err := validateTemplateSpecVersion(ctx, httpClient, raw)
			if err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"valid\t%t\nprimary_output_type\t%s\ndefinition_hash\t%s\ncontract_bundle_hash\t%s\nauthority_fingerprint\t%s\n",
				result.Valid, result.PrimaryOutputType, result.DefinitionHash,
				result.ContractBundleHash, result.AuthorityFingerprint,
			)
			return err
		},
	}
}

func newTemplateSpecModelsCmd(opts *rootOptions) *cobra.Command {
	var provider string
	var includeUnavailable bool

	cmd := &cobra.Command{
		Use:   "models <step-type>",
		Short: "List authority-backed models usable for a TemplateSpec step type",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stepType := strings.TrimSpace(args[0])
			if stepType == "" {
				return errors.New("step-type is required")
			}
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			query := url.Values{}
			query.Set("stepType", stepType)
			if includeUnavailable {
				return errors.New("--include-unavailable is no longer supported: LoomLoom lists only models with an authoring contract")
			}

			var resp listModelsResponse
			if err := httpClient.GetJSONWithQuery(ctx, "/models", query, &resp); err != nil {
				return err
			}
			resp.Models = filterModelsByProvider(resp.Models, provider)
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}
			return printTemplateSpecModels(cmd.OutOrStdout(), resp.Models)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Optional client-side model provider filter")
	cmd.Flags().BoolVar(&includeUnavailable, "include-unavailable", false, "Deprecated: LoomLoom no longer exposes unavailable catalog models")
	return cmd
}

func newTemplateSpecCreateCmd(opts *rootOptions) *cobra.Command {
	var name string
	var description string
	var versionNote string

	cmd := &cobra.Command{
		Use:   "create <spec-json>",
		Short: "Create a private user template and save the TemplateSpec JSON as version 1",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := loadTemplateSpecTransportFile(args[0])
			if err != nil {
				return err
			}
			var authoringMeta struct {
				Meta struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"meta"`
			}
			if err := json.Unmarshal(raw, &authoringMeta); err != nil {
				return fmt.Errorf("decode TemplateSpec metadata: %w", err)
			}
			effectiveName := firstNonEmpty(name, authoringMeta.Meta.Name)
			if effectiveName == "" {
				return errors.New("template name is required; set meta.name or pass --name")
			}
			effectiveDescription := firstNonEmpty(description, authoringMeta.Meta.Description)

			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			if _, err := validateTemplateSpecVersion(ctx, httpClient, raw); err != nil {
				return fmt.Errorf("validate template spec: %w", err)
			}

			var createResp createUserTemplateResponse
			if err := httpClient.PostJSON(ctx, "/users/me/templates", map[string]any{
				"name":        effectiveName,
				"description": effectiveDescription,
			}, &createResp); err != nil {
				return err
			}

			versionResp, err := saveTemplateSpecVersion(ctx, httpClient, createResp.TemplateID, raw, versionNote)
			if err != nil {
				return fmt.Errorf("save template version for %s: %w", createResp.TemplateID, err)
			}

			result := map[string]any{
				"templateId":     createResp.TemplateID,
				"name":           createResp.Name,
				"description":    createResp.Description,
				"status":         createResp.Status,
				"versionId":      versionResp.VersionID,
				"versionNumber":  int64(versionResp.VersionNumber),
				"definitionHash": versionResp.DefinitionHash,
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"template_id\t%s\nname\t%s\nversion_id\t%s\nversion_number\t%d\ndefinition_hash\t%s\n",
				createResp.TemplateID,
				createResp.Name,
				versionResp.VersionID,
				int64(versionResp.VersionNumber),
				versionResp.DefinitionHash,
			)
			return err
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Template name override; defaults to meta.name")
	cmd.Flags().StringVar(&description, "description", "", "Template description override; defaults to meta.description")
	cmd.Flags().StringVar(&versionNote, "version-note", "", "Optional note for version 1")
	return cmd
}

func newTemplateSpecCreateVersionCmd(opts *rootOptions) *cobra.Command {
	var versionNote string

	cmd := &cobra.Command{
		Use:   "create-version <template-id> <spec-json>",
		Short: "Create a new immutable version for an existing user template",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := strings.TrimSpace(args[0])
			if templateID == "" {
				return errors.New("template ID is required")
			}
			raw, err := loadTemplateSpecTransportFile(args[1])
			if err != nil {
				return err
			}

			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			if _, err := validateTemplateSpecVersion(ctx, httpClient, raw); err != nil {
				return fmt.Errorf("validate template spec: %w", err)
			}

			versionResp, err := saveTemplateSpecVersion(ctx, httpClient, templateID, raw, versionNote)
			if err != nil {
				return fmt.Errorf("save template version for %s: %w", templateID, err)
			}

			result := map[string]any{
				"templateId":     templateID,
				"versionId":      versionResp.VersionID,
				"versionNumber":  int64(versionResp.VersionNumber),
				"definitionHash": versionResp.DefinitionHash,
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"template_id\t%s\nversion_id\t%s\nversion_number\t%d\ndefinition_hash\t%s\n",
				templateID,
				versionResp.VersionID,
				int64(versionResp.VersionNumber),
				versionResp.DefinitionHash,
			)
			return err
		},
	}
	cmd.Flags().StringVar(&versionNote, "version-note", "", "Optional note for the new version")
	return cmd
}

func saveTemplateSpecVersion(ctx context.Context, httpClient *client.Client, templateID string, rawSpec []byte, versionNote string) (saveTemplateVersionResponse, error) {
	var versionResp saveTemplateVersionResponse
	err := httpClient.PostJSON(ctx, "/users/me/templates/"+templateID+"/versions", map[string]any{
		"versionNote":     strings.TrimSpace(versionNote),
		"specVersion":     "template-spec/v2",
		"canonicalSpecV2": json.RawMessage(rawSpec),
	}, &versionResp)
	return versionResp, err
}

func validateTemplateSpecVersion(ctx context.Context, httpClient *client.Client, rawSpec []byte) (validateTemplateSpecResponse, error) {
	var resp validateTemplateSpecResponse
	err := httpClient.PostProductJSON(ctx, "/templateSpecs:validate", map[string]any{
		"specVersion": "template-spec/v2", "canonicalSpecV2": json.RawMessage(rawSpec),
	}, &resp)
	if err != nil {
		return validateTemplateSpecResponse{}, err
	}
	if !resp.Valid {
		return validateTemplateSpecResponse{}, errors.New("server returned valid=false without an error")
	}
	return resp, nil
}

func newTemplateSpecDownloadWorkbookCmd(opts *rootOptions) *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "download-workbook <template-id> <version-id>",
		Short: "Download the Excel workbook generated from a user template version",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			templateID := strings.TrimSpace(args[0])
			versionID := strings.TrimSpace(args[1])
			resp, err := httpClient.GetBinary(ctx, "/users/me/templates/"+templateID+"/versions/"+versionID+"/workbook")
			if err != nil {
				return err
			}
			filename := suggestedDownloadFilename(resp.ContentDisposition)
			if filename == "" {
				filename = templateID + "-" + versionID + ".xlsx"
			}
			targetPath, err := resolveFilePath(outputPath, filepath.Base(filename))
			if err != nil {
				return fmt.Errorf("resolve output file path: %w", err)
			}
			if err := os.WriteFile(targetPath, resp.Body, 0o644); err != nil {
				return fmt.Errorf("write downloaded file: %w", err)
			}
			result := map[string]any{
				"templateId": templateID,
				"versionId":  versionID,
				"path":       targetPath,
				"filename":   filename,
				"size":       len(resp.Body),
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "template_id\t%s\nversion_id\t%s\npath\t%s\nsize\t%d\n", templateID, versionID, targetPath, len(resp.Body))
			return err
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output-file", "f", "", "Output .xlsx path or target directory")
	return cmd
}

func newTemplateSpecValidateWorkbookCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "validate-workbook <template-id> <version-id> <xlsx-path>",
		Short: "Validate a filled user-template workbook",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint := "/users/me/templates/" + strings.TrimSpace(args[0]) + "/versions/" + strings.TrimSpace(args[1]) + ":validateWorkbook"
			resp, err := postUserTemplateWorkbook[validateTemplateFileResponse](cmd.Context(), opts, args[2], endpoint, nil)
			if err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"templateId": args[0],
					"versionId":  args[1],
					"file":       args[2],
					"validation": resp,
				})
			}
			if err := printTemplateFileValidation(cmd.OutOrStdout(), resp); err != nil {
				return err
			}
			if !resp.Valid {
				return templateFileValidationError(resp)
			}
			return nil
		},
	}
}

func newTemplateSpecPrecheckWorkbookCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "precheck-workbook <template-id> <version-id> <xlsx-path>",
		Short: "Estimate cost for a user-template workbook without submitting",
		Long:  "Estimate cost for a user-template workbook without submitting. The command uses a 10 minute timeout unless --timeout is explicitly supplied.",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := strings.TrimSpace(args[0])
			versionID := strings.TrimSpace(args[1])
			endpoint := "/users/me/templates/" + url.PathEscape(templateID) + "/versions/" + url.PathEscape(versionID) + ":precheckWorkbook"
			requestOpts := precheckRootOptions(cmd, opts)
			resp, err := postUserTemplateWorkbook[precheckTemplateRowsResponse](cmd.Context(), requestOpts, args[2], endpoint, nil)
			if err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(map[string]any{
					"templateId": templateID,
					"versionId":  versionID,
					"file":       args[2],
					"precheck":   precheckJSONPayload(resp),
				}); err != nil {
					return err
				}
				return printInsufficientBalanceHint(cmd, opts, resp.BalanceCheck)
			}
			if err := printPrecheck(cmd.OutOrStdout(), resp); err != nil {
				return err
			}
			return printInsufficientBalanceHint(cmd, opts, resp.BalanceCheck)
		},
	}
}

func newTemplateSpecSubmitWorkbookCmd(opts *rootOptions) *cobra.Command {
	var callbackURL string
	var clientRequestID string

	cmd := &cobra.Command{
		Use:   "submit-workbook <template-id> <version-id> <xlsx-path>",
		Short: "Submit a filled user-template workbook as a run",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			validateEndpoint := "/users/me/templates/" + strings.TrimSpace(args[0]) + "/versions/" + strings.TrimSpace(args[1]) + ":validateWorkbook"
			validateResp, err := postUserTemplateWorkbook[validateTemplateFileResponse](cmd.Context(), opts, args[2], validateEndpoint, nil)
			if err != nil {
				return err
			}
			if !validateResp.Valid {
				if opts.output == "json" {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					_ = enc.Encode(map[string]any{
						"templateId": args[0],
						"versionId":  args[1],
						"file":       args[2],
						"validation": validateResp,
					})
				}
				return templateFileValidationError(validateResp)
			}

			crid, generatedRequestID := effectiveClientRequestID(clientRequestID)
			extra := map[string]string{
				"clientRequestId": crid,
			}
			if strings.TrimSpace(callbackURL) != "" {
				extra["callbackUrl"] = strings.TrimSpace(callbackURL)
			}
			submitEndpoint := "/users/me/templates/" + strings.TrimSpace(args[0]) + "/versions/" + strings.TrimSpace(args[1]) + ":runWorkbook"
			printGeneratedClientRequestID(cmd, crid, generatedRequestID)
			submitResp, err := postUserTemplateWorkbook[submitUserTemplateWorkbookResponse](cmd.Context(), opts, args[2], submitEndpoint, extra)
			if err != nil {
				return err
			}
			result := map[string]any{
				"templateId":      args[0],
				"versionId":       args[1],
				"file":            args[2],
				"clientRequestId": crid,
				"runId":           submitResp.RunID,
				"status":          submitResp.Status,
				"acceptedAt":      int64(submitResp.AcceptedAt),
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"template_id\t%s\nversion_id\t%s\nfile\t%s\nrun_id\t%s\nstatus\t%s\naccepted_at\t%s\n",
				args[0],
				args[1],
				args[2],
				submitResp.RunID,
				submitResp.Status,
				formatUnix(int64(submitResp.AcceptedAt)),
			)
			return err
		},
	}
	cmd.Flags().StringVar(&callbackURL, "callback-url", "", "Optional callback URL")
	cmd.Flags().StringVar(&clientRequestID, "client-request-id", "", "Stable idempotency key for retrying the same workbook submission")
	cmd.Flags().StringVar(&clientRequestID, "idempotency-key", "", "Deprecated alias for --client-request-id")
	_ = cmd.Flags().MarkDeprecated("idempotency-key", "use --client-request-id")
	return cmd
}

func postUserTemplateWorkbook[T any](ctx context.Context, opts *rootOptions, workbookPath, endpoint string, extra map[string]string) (T, error) {
	var zero T
	payload, err := workbookPayload(workbookPath, extra)
	if err != nil {
		return zero, err
	}
	httpClient, err := newHTTPClient(opts)
	if err != nil {
		return zero, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	var out T
	if err := httpClient.PostJSON(requestCtx, endpoint, payload, &out); err != nil {
		return zero, err
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func printTemplateSpecModels(w interface {
	Write([]byte) (int, error)
}, models []modelSummary) error {
	if len(models) == 0 {
		_, err := fmt.Fprintln(w, "no models")
		return err
	}
	tw := newTabWriter(w)
	if _, err := fmt.Fprintln(tw, "model_id\tdisplay_name\tprovider\tauthoring_options"); err != nil {
		return err
	}
	for _, model := range models {
		kinds := make([]string, 0, len(model.AuthoringOptions))
		for _, option := range model.AuthoringOptions {
			kinds = append(kinds, option.Kind)
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\n",
			model.ModelID,
			model.DisplayName,
			modelProvider(model),
			strings.Join(kinds, ","),
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func modelProvider(value modelSummary) string {
	provider, _, _ := strings.Cut(value.ModelID, "/")
	return provider
}

func filterModelsByProvider(models []modelSummary, provider string) []modelSummary {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return models
	}
	filtered := make([]modelSummary, 0, len(models))
	for _, item := range models {
		if modelProvider(item) == provider {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func newTemplateSpecListCmd(opts *rootOptions) *cobra.Command {
	var (
		status     string
		pageSize   int
		pageOffset int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List my private templates",
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
			if pageOffset > 0 {
				query.Set("pageOffset", fmt.Sprintf("%d", pageOffset))
			}

			var resp map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/users/me/templates", query, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by template status (default: active)")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Page size")
	cmd.Flags().IntVar(&pageOffset, "page-offset", 0, "Page offset")
	return cmd
}

func newTemplateSpecGetCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <template-id>",
		Short: "Get one private template with its version list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			path := "/users/me/templates/" + url.PathEscape(strings.TrimSpace(args[0]))
			var resp map[string]any
			if err := httpClient.GetProductJSON(ctx, path, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
}

func newTemplateSpecVersionsCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "versions <template-id>",
		Short: "List versions of a private template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			path := "/users/me/templates/" + url.PathEscape(strings.TrimSpace(args[0])) + "/versions"
			var resp map[string]any
			if err := httpClient.GetProductJSON(ctx, path, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
}

func newTemplateSpecGetVersionCmd(opts *rootOptions) *cobra.Command {
	var outputPath string
	cmd := &cobra.Command{
		Use:   "get-version <template-id> <version-id>",
		Short: "Get one historical private TemplateSpec authoring definition",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID, versionID := strings.TrimSpace(args[0]), strings.TrimSpace(args[1])
			if templateID == "" || versionID == "" {
				return errors.New("template ID and version ID are required")
			}
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			endpoint := "/users/me/templates/" + url.PathEscape(templateID) + "/versions/" + url.PathEscape(versionID)
			var resp templateVersionSpecResponse
			if err := httpClient.GetProductJSON(ctx, endpoint, &resp); err != nil {
				return err
			}
			if len(resp.CanonicalSpec) == 0 || !json.Valid(resp.CanonicalSpec) {
				return errors.New("server returned an invalid canonicalSpec")
			}
			if strings.TrimSpace(outputPath) != "" {
				targetPath, err := resolveFilePath(outputPath, templateID+"-"+versionID+".json")
				if err != nil {
					return fmt.Errorf("resolve output file path: %w", err)
				}
				var pretty bytes.Buffer
				if err := json.Indent(&pretty, resp.CanonicalSpec, "", "  "); err != nil {
					return fmt.Errorf("format canonicalSpec: %w", err)
				}
				pretty.WriteByte('\n')
				if err := os.WriteFile(targetPath, pretty.Bytes(), 0o644); err != nil {
					return fmt.Errorf("write TemplateSpec: %w", err)
				}
				if opts.output == "json" {
					return writeIndentedJSON(cmd.OutOrStdout(), map[string]any{
						"templateId": resp.TemplateID, "versionId": resp.VersionID, "specVersion": resp.SpecVersion,
						"definitionHash": resp.DefinitionHash, "path": targetPath,
					})
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "template_id\t%s\nversion_id\t%s\nspec_version\t%s\npath\t%s\n", resp.TemplateID, resp.VersionID, resp.SpecVersion, targetPath)
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output-file", "f", "", "Write canonicalSpec to a JSON file or target directory")
	return cmd
}

func newTemplateSpecPrecheckCmd(opts *rootOptions) *cobra.Command {
	var (
		versionID   string
		inputFileID string
	)
	cmd := &cobra.Command{
		Use:   "precheck <template-id>",
		Short: "Estimate cost for a private template JSONL input without submitting",
		Long:  "Estimate cost for a private template JSONL input without submitting. The command uses a 10 minute timeout unless --timeout is explicitly supplied.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trimmedInputFileID, err := normalizeTemplateSpecInputFileID(inputFileID)
			if err != nil {
				return err
			}

			requestOpts := precheckRootOptions(cmd, opts)
			httpClient, err := newHTTPClient(requestOpts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), requestOpts.timeout)
			defer cancel()

			templateID := strings.TrimSpace(args[0])
			trimmedVersionID := strings.TrimSpace(versionID)
			payload := map[string]any{
				"versionId":   trimmedVersionID,
				"inputFileId": trimmedInputFileID,
			}

			path := "/users/me/templates/" + url.PathEscape(templateID) + ":precheck"
			var resp precheckTemplateRowsResponse
			if err := httpClient.PostProductJSON(ctx, path, payload, &resp); err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(map[string]any{
					"templateId":  templateID,
					"versionId":   trimmedVersionID,
					"inputFileId": trimmedInputFileID,
					"precheck":    precheckJSONPayload(resp),
				}); err != nil {
					return err
				}
				return printInsufficientBalanceHint(cmd, opts, resp.BalanceCheck)
			}
			if err := printPrecheck(cmd.OutOrStdout(), resp); err != nil {
				return err
			}
			return printInsufficientBalanceHint(cmd, opts, resp.BalanceCheck)
		},
	}
	cmd.Flags().StringVar(&versionID, "version-id", "", "Template version ID to precheck")
	cmd.Flags().StringVar(&inputFileID, "input-file-id", "", "Execution input fileId returned by orchestrationInputs:upload (not inputAssets:upload)")
	_ = cmd.MarkFlagRequired("version-id")
	_ = cmd.MarkFlagRequired("input-file-id")
	return cmd
}

func newTemplateSpecEstimateCmd(opts *rootOptions) *cobra.Command {
	var versionID, inputFileID string
	cmd := &cobra.Command{
		Use:   "estimate <template-id>",
		Short: "Estimate private template JSONL cost without validating referenced resources",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trimmedInputFileID, err := normalizeTemplateSpecInputFileID(inputFileID)
			if err != nil {
				return err
			}
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			templateID, trimmedVersionID := strings.TrimSpace(args[0]), strings.TrimSpace(versionID)
			var resp precheckTemplateRowsResponse
			if err := httpClient.PostProductJSON(ctx, "/users/me/templates/"+url.PathEscape(templateID)+":estimate", map[string]any{
				"versionId": trimmedVersionID, "inputFileId": trimmedInputFileID,
			}, &resp); err != nil {
				return err
			}
			if opts.output == "json" {
				payload := precheckJSONPayload(resp)
				delete(payload, "balanceCheck")
				return writeIndentedJSON(cmd.OutOrStdout(), map[string]any{
					"templateId": templateID, "versionId": trimmedVersionID, "inputFileId": trimmedInputFileID,
					"resourcesValidated": false, "estimate": payload,
				})
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "resources_validated\tfalse"); err != nil {
				return err
			}
			return printPrecheck(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&versionID, "version-id", "", "Template version ID to estimate")
	cmd.Flags().StringVar(&inputFileID, "input-file-id", "", "Execution input fileId returned by orchestrationInputs:upload")
	_ = cmd.MarkFlagRequired("version-id")
	_ = cmd.MarkFlagRequired("input-file-id")
	return cmd
}

const defaultPrecheckHTTPTimeout = 10 * time.Minute

func precheckRootOptions(cmd *cobra.Command, opts *rootOptions) *rootOptions {
	cloned := *opts
	if !flagChanged(cmd, "timeout") {
		cloned.timeout = defaultPrecheckHTTPTimeout
	}
	return &cloned
}

func printInsufficientBalanceHint(cmd *cobra.Command, opts *rootOptions, balance *templateBalanceCheck) error {
	if balance == nil || balance.IsSufficient {
		return nil
	}
	if message := insufficientBalanceMessage(opts); message != "" {
		_, err := fmt.Fprintln(cmd.ErrOrStderr(), message)
		return err
	}
	return nil
}

func newTemplateSpecRunCmd(opts *rootOptions) *cobra.Command {
	var (
		versionID       string
		inputFileID     string
		clientRequestID string
		callbackURL     string
	)
	cmd := &cobra.Command{
		Use:   "run <template-id>",
		Short: "Submit a run for a private template version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trimmedInputFileID, err := normalizeTemplateSpecInputFileID(inputFileID)
			if err != nil {
				return err
			}

			crid, generatedRequestID := effectiveClientRequestID(clientRequestID)

			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			payload := map[string]any{
				"versionId":       strings.TrimSpace(versionID),
				"inputFileId":     trimmedInputFileID,
				"clientRequestId": crid,
			}
			if strings.TrimSpace(callbackURL) != "" {
				payload["callbackUrl"] = strings.TrimSpace(callbackURL)
			}

			path := "/users/me/templates/" + url.PathEscape(strings.TrimSpace(args[0])) + ":run"
			printGeneratedClientRequestID(cmd, crid, generatedRequestID)
			var resp map[string]any
			if err := httpClient.PostProductJSON(ctx, path, payload, &resp); err != nil {
				return err
			}
			opts.debugf(
				"private template run: submitted template_id=%s version_id=%s input_file_id=%s run_id=%s",
				strings.TrimSpace(args[0]),
				strings.TrimSpace(versionID),
				trimmedInputFileID,
				stringMapValue(resp, "runId"),
			)
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().StringVar(&versionID, "version-id", "", "Template version ID to run")
	cmd.Flags().StringVar(&inputFileID, "input-file-id", "", "Execution input fileId returned by orchestrationInputs:upload (not inputAssets:upload)")
	cmd.Flags().StringVar(&clientRequestID, "client-request-id", "", "Idempotency key; auto-generated if omitted")
	cmd.Flags().StringVar(&callbackURL, "callback-url", "", "Optional callback URL")
	_ = cmd.MarkFlagRequired("version-id")
	_ = cmd.MarkFlagRequired("input-file-id")
	return cmd
}

func normalizeTemplateSpecInputFileID(inputFileID string) (string, error) {
	trimmedInputFileID := strings.TrimSpace(inputFileID)
	if strings.HasPrefix(strings.ToLower(trimmedInputFileID), "ia_") {
		return "", fmt.Errorf("--input-file-id requires the fileId returned by orchestrationInputs:upload; inputAssets:upload returns an inputAssetId (%q) that cannot be used to run a template", trimmedInputFileID)
	}
	return trimmedInputFileID, nil
}
