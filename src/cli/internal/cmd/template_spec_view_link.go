package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cogfoundry-labs/loomloom/src/cli/internal/client"
	"github.com/cogfoundry-labs/loomloom/src/cli/internal/workflowpreview"
	"github.com/spf13/cobra"
	"net/url"
	"strings"
	"time"
)

type templateVersionViewLink struct {
	TemplateID string `json:"templateId"`
	VersionID  string `json:"versionId"`
	ViewURL    string `json:"viewUrl"`
}

func newTemplateSpecViewLinkCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "view-link <template-id> <version-id>",
		Short: "Get the owner-only workbench link for an exact saved private version",
		Long: `Return the Server-provided link for one saved private template version.
Pass actual IDs from get/versions or create/create-version, not a latest alias.
This does not upload files, start a preview, change the version, or run it.
The browser must sign in as the same owner. No CLI process needs to stay running.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, id := range args {
				if strings.TrimSpace(id) == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\?#\r\n\t") {
					return fmt.Errorf("template and version must be concrete resource IDs")
				}
			}
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			response, err := readTemplateVersionViewLink(ctx, httpClient, args[0], args[1])
			if err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(response)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), response.ViewURL)
			return err
		},
	}
}

// readTemplateVersionViewLink never derives a URL locally or returns another version's link.
func readTemplateVersionViewLink(ctx context.Context, httpClient *client.Client, templateID, versionID string) (templateVersionViewLink, error) {
	var response templateVersionViewLink
	path := "/users/me/templates/" + url.PathEscape(templateID) + "/versions/" + url.PathEscape(versionID) + "/viewLink"
	if err := httpClient.GetJSON(ctx, path, &response); err != nil {
		return response, err
	}
	if response.TemplateID != templateID || response.VersionID != versionID {
		return templateVersionViewLink{}, fmt.Errorf("server returned a link for a different template version")
	}
	if err := workflowpreview.ValidateViewURL(response.ViewURL); err != nil {
		return templateVersionViewLink{}, err
	}
	return response, nil
}

// A saved version is durable before this optional read. Never report a failed
// save or retry a mutation because the viewing service is unavailable.
func appendSavedVersionViewLink(cmd *cobra.Command, opts *rootOptions, httpClient *client.Client, result map[string]any, templateID, versionID string) {
	timeout := min(opts.timeout, 10*time.Second)
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()
	link, err := readTemplateVersionViewLink(ctx, httpClient, templateID, versionID)
	if err != nil {
		result["viewLinkError"] = "view_link_unavailable"
		// Avoid copying server error bodies, credentials or definition text into warnings.
		fmt.Fprintln(cmd.ErrOrStderr(), "Version saved successfully; its view link is unavailable. Retry template-spec view-link with the returned templateId and versionId. Do not repeat the save.")
		return
	}
	result["viewUrl"] = link.ViewURL
}
