package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type skillPackageDownloadResult struct {
	TemplateID            string `json:"templateId,omitempty"`
	ListingID             string `json:"listingId,omitempty"`
	SkillPackageVersionID string `json:"skillPackageVersionId,omitempty"`
	Path                  string `json:"path"`
	Filename              string `json:"filename"`
	Size                  int    `json:"size"`
}

func newSkillPackagePrivateDownloadCmd(opts *rootOptions) *cobra.Command {
	var outputPath string
	cmd := &cobra.Command{
		Use:   "download <template-id>",
		Short: "Download the current private template Skill Package ZIP without installing it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.TrimSpace(args[0])
			if id == "" {
				return fmt.Errorf("template ID is required")
			}
			return downloadSkillPackageArchive(cmd, opts,
				"/users/me/templates/"+url.PathEscape(id)+"/skillPackage/archive",
				outputPath, "private-skill-package.zip", skillPackageDownloadResult{TemplateID: id})
		},
	}
	addSkillPackageDownloadFlag(cmd, &outputPath)
	return cmd
}

func newListingDownloadSkillPackageCmd(opts *rootOptions) *cobra.Command {
	var outputPath, versionID string
	cmd := &cobra.Command{
		Use:   "download-skill-package <listing-id>",
		Short: "Download one of your Listing's Skill Package version ZIPs without installing it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, version := strings.TrimSpace(args[0]), strings.TrimSpace(versionID)
			if id == "" || version == "" {
				return fmt.Errorf("listing ID and --version-id (Skill Package version ID) are required")
			}
			return downloadSkillPackageArchive(cmd, opts,
				"/creators/me/marketListings/"+url.PathEscape(id)+"/skillPackageVersions/"+url.PathEscape(version)+"/archive",
				outputPath, "listing-skill-package.zip", skillPackageDownloadResult{ListingID: id, SkillPackageVersionID: version})
		},
	}
	cmd.Flags().StringVar(&versionID, "version-id", "", "Skill Package version ID, not a Listing version or template version ID")
	_ = cmd.MarkFlagRequired("version-id")
	addSkillPackageDownloadFlag(cmd, &outputPath)
	return cmd
}

func addSkillPackageDownloadFlag(cmd *cobra.Command, outputPath *string) {
	cmd.Flags().StringVarP(outputPath, "output-file", "f", "", "Output ZIP path or target directory; replaces an existing file after download succeeds")
	_ = cmd.MarkFlagRequired("output-file")
}

func downloadSkillPackageArchive(cmd *cobra.Command, opts *rootOptions, archivePath, outputPath, fallbackName string, result skillPackageDownloadResult) error {
	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("--output-file is required")
	}
	httpClient, err := newHTTPClient(opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
	defer cancel()
	archive, err := httpClient.GetBinary(ctx, archivePath)
	if err != nil {
		return err
	}
	filename := suggestedDownloadFilename(archive.ContentDisposition)
	if filename == "" || filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\") {
		filename = fallbackName
	}
	target, err := resolveFilePath(outputPath, filename)
	if err != nil {
		return fmt.Errorf("resolve output file path: %w", err)
	}
	if err := writeSkillPackageArchive(target, archive.Body); err != nil {
		return fmt.Errorf("save Skill Package ZIP: %w", err)
	}
	result.Path, result.Filename, result.Size = target, filepath.Base(target), len(archive.Body)
	if opts.output == "json" {
		return writeIndentedJSON(cmd.OutOrStdout(), result)
	}
	if result.TemplateID != "" {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "template_id\t%s\npath\t%s\nsize\t%d\n", result.TemplateID, target, result.Size)
	} else {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "listing_id\t%s\nskill_package_version_id\t%s\npath\t%s\nsize\t%d\n", result.ListingID, result.SkillPackageVersionID, target, result.Size)
	}
	return err
}

// Stage beside the destination so a failed write cannot truncate an existing ZIP.
func writeSkillPackageArchive(target string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(target), ".loomloom-skill-download-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), target)
}
