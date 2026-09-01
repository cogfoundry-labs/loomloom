package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/client"
	"github.com/cogfoundry-labs/loomloom/src/cli/internal/skill"
	"github.com/spf13/cobra"
)

type skillPackageSummary struct {
	Available             bool   `json:"available"`
	UnavailableReason     string `json:"unavailableReason"`
	SkillPackageVersionID string `json:"skillPackageVersionId"`
	Mode                  string `json:"mode"`
	ArchiveHash           string `json:"archiveHash"`
	SizeBytes             int64  `json:"sizeBytes"`
	TotalDownloadCount    int64  `json:"totalDownloadCount"`
}

func newSkillPackageCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "package", Short: "Manage backend-provided Agent Skill ZIP packages"}
	install := &cobra.Command{Use: "install", Short: "Download and install or update a Skill ZIP in the current Agent's Skill root"}
	install.AddCommand(newSkillPackageInstallMarketCmd(opts), newSkillPackageInstallOfficialCmd(opts))
	private := &cobra.Command{Use: "private", Short: "Manage the current private template Skill ZIP"}
	private.AddCommand(newSkillPackagePrivateUploadCmd(opts), newSkillPackagePrivateShowCmd(opts), newSkillPackagePrivateDetachCmd(opts))
	cmd.AddCommand(install, private)
	return cmd
}

func addSkillRootFlag(cmd *cobra.Command, root *string) {
	cmd.Flags().StringVar(root, "skill-root", "", "Current Agent's Skill root; the package folder will be installed beneath it")
	_ = cmd.MarkFlagRequired("skill-root")
}

func newSkillPackageInstallMarketCmd(opts *rootOptions) *cobra.Command {
	var root string
	cmd := &cobra.Command{Use: "market <listing-id>", Short: "Install or update a Market SkillBot ZIP package", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		listingID := strings.TrimSpace(args[0])
		listingPath := url.PathEscape(listingID)
		return installPublicSkillPackage(cmd, opts, "/marketListings/"+listingPath+"/skillPackage", "/marketListings/"+listingPath+"/skillPackage/archive", root, "market:"+listingID)
	}}
	addSkillRootFlag(cmd, &root)
	return cmd
}

func newSkillPackageInstallOfficialCmd(opts *rootOptions) *cobra.Command {
	var root string
	cmd := &cobra.Command{Use: "official <template-slug>", Short: "Install or update an official template ZIP package", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		slug := strings.TrimSpace(args[0])
		return installOfficialSkillPackage(cmd, opts, slug, root)
	}}
	addSkillRootFlag(cmd, &root)
	return cmd
}

func installPublicSkillPackage(cmd *cobra.Command, opts *rootOptions, summaryPath, archivePath, root, sourceRef string) error {
	httpClient, err := newHTTPClient(opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
	defer cancel()
	var summary skillPackageSummary
	if err := httpClient.GetJSON(ctx, summaryPath, &summary); err != nil {
		return err
	}
	return downloadAndInstallSkillPackage(cmd, ctx, httpClient, summary, archivePath, root, sourceRef)
}

func installOfficialSkillPackage(cmd *cobra.Command, opts *rootOptions, slug, root string) error {
	httpClient, err := newHTTPClient(opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
	defer cancel()
	var detail struct {
		SkillPackage skillPackageSummary `json:"skillPackage"`
	}
	escapedSlug := url.PathEscape(slug)
	if err := httpClient.GetJSON(ctx, "/officialTemplates/"+escapedSlug+"/schema", &detail); err != nil {
		return err
	}
	return downloadAndInstallSkillPackage(cmd, ctx, httpClient, detail.SkillPackage, "/officialTemplates/"+escapedSlug+"/skillPackage/archive", root, "official:"+slug)
}

func downloadAndInstallSkillPackage(cmd *cobra.Command, ctx context.Context, httpClient interface {
	GetBinary(context.Context, string) (*client.BinaryResponse, error)
}, summary skillPackageSummary, archivePath, root, sourceRef string) error {
	if !summary.Available {
		return writePackageUnavailableSummary(cmd, summary)
	}
	expectedHash := strings.TrimSpace(summary.ArchiveHash)
	if expectedHash == "" {
		return fmt.Errorf("public skill package summary is missing archiveHash")
	}
	if result, unchanged, err := skill.FindInstalledPackage(root, sourceRef, summary.ArchiveHash); err != nil {
		return err
	} else if unchanged {
		return writePackageInstallResult(cmd, result)
	}
	archive, err := httpClient.GetBinary(ctx, archivePath)
	if err != nil {
		return err
	}
	result, err := skill.InstallPackage(skill.PackageInstallOptions{SkillRoot: root, SourceRef: sourceRef, ArchiveHash: expectedHash, Archive: archive.Body})
	if err != nil {
		return err
	}
	return writePackageInstallResult(cmd, result)
}

func writePackageUnavailableSummary(cmd *cobra.Command, summary skillPackageSummary) error {
	if optsOutputJSON(cmd) {
		return writeIndentedJSON(cmd.OutOrStdout(), summary)
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "available\t%t\nunavailable_reason\t%s\n", summary.Available, summary.UnavailableReason)
	return err
}

func writePackageInstallResult(cmd *cobra.Command, result *skill.PackageInstallResult) error {
	if optsOutputJSON(cmd) {
		return writeIndentedJSON(cmd.OutOrStdout(), result)
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "installed\t%t\nupdated\t%t\nunchanged\t%t\nskill_name\t%s\ndir\t%s\narchive_hash\t%s\n", result.Installed, result.Updated, result.Unchanged, result.SkillName, result.Dir, result.ArchiveHash)
	return err
}

func newSkillPackagePrivateUploadCmd(opts *rootOptions) *cobra.Command {
	var file, expectedArchiveHash, expectedValidationID string
	cmd := &cobra.Command{Use: "upload <template-id>", Short: "Upload or replace a private template Skill ZIP", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		httpClient, err := newHTTPClient(opts)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
		defer cancel()
		query, err := packageCASQuery(expectedArchiveHash, expectedValidationID)
		if err != nil {
			return err
		}
		var response map[string]any
		path := "/users/me/templates/" + url.PathEscape(strings.TrimSpace(args[0])) + "/skillPackage"
		if len(query) > 0 {
			path += "?" + query.Encode()
		}
		if err := httpClient.PutMultipartFile(ctx, path, nil, "file", strings.TrimSpace(file), &response); err != nil {
			return err
		}
		return writeIndentedJSON(cmd.OutOrStdout(), response)
	}}
	cmd.Flags().StringVar(&file, "file", "", "Path to the Agent-created Skill ZIP")
	cmd.Flags().StringVar(&expectedArchiveHash, "expected-archive-hash", "", "Current Head archive hash for compare-and-swap")
	cmd.Flags().StringVar(&expectedValidationID, "expected-validation-id", "", "Current Head validation ID for compare-and-swap")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newSkillPackagePrivateShowCmd(opts *rootOptions) *cobra.Command {
	return privatePackageJSONCmd(opts, "show <template-id>", "Show the private template Skill Package Head", func(id string) string { return "/users/me/templates/" + url.PathEscape(id) + "/skillPackage" })
}

func newSkillPackagePrivateDetachCmd(opts *rootOptions) *cobra.Command {
	var expectedArchiveHash, expectedValidationID string
	cmd := &cobra.Command{Use: "detach <template-id>", Short: "Detach the current Skill Package from a private template", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		httpClient, err := newHTTPClient(opts)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
		defer cancel()
		var response map[string]any
		query, err := packageCASQuery(expectedArchiveHash, expectedValidationID)
		if err != nil {
			return err
		}
		if err := httpClient.DeleteProduct(ctx, "/users/me/templates/"+url.PathEscape(strings.TrimSpace(args[0]))+"/skillPackage", query, &response); err != nil {
			return err
		}
		return writeIndentedJSON(cmd.OutOrStdout(), response)
	}}
	cmd.Flags().StringVar(&expectedArchiveHash, "expected-archive-hash", "", "Current bound package archive hash for compare-and-swap")
	cmd.Flags().StringVar(&expectedValidationID, "expected-validation-id", "", "Current bound package validation ID for compare-and-swap")
	return cmd
}

func privatePackageJSONCmd(opts *rootOptions, use, short string, path func(string) string) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		httpClient, err := newHTTPClient(opts)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
		defer cancel()
		var response map[string]any
		if err := httpClient.GetJSON(ctx, path(strings.TrimSpace(args[0])), &response); err != nil {
			return err
		}
		return writeIndentedJSON(cmd.OutOrStdout(), response)
	}}
}

func packageCASQuery(archiveHash, validationID string) (url.Values, error) {
	values := url.Values{}
	archiveHash = strings.TrimSpace(archiveHash)
	validationID = strings.TrimSpace(validationID)
	if (archiveHash == "") != (validationID == "") {
		return nil, fmt.Errorf("--expected-archive-hash and --expected-validation-id must be provided together")
	}
	if archiveHash != "" {
		values.Set("expectedArchiveHash", archiveHash)
		values.Set("expectedValidationId", validationID)
	}
	return values, nil
}
func optsOutputJSON(cmd *cobra.Command) bool {
	output, _ := cmd.Root().PersistentFlags().GetString("output")
	return strings.EqualFold(strings.TrimSpace(output), "json")
}
