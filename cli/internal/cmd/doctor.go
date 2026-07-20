package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/client"
	"github.com/Cogfoundry-ai/loomloom/cli/internal/platform"
	"github.com/Cogfoundry-ai/loomloom/cli/internal/version"
	"github.com/spf13/cobra"
)

func newDoctorCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check LoomLoom server reachability and token wiring",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedPlatform, err := resolvePlatform(opts)
			if err != nil {
				return err
			}
			if strings.TrimSpace(opts.server) == "" {
				return writeDoctorCredentialOutput(cmd, opts, platform.UnknownPlatform())
			}
			if strings.TrimSpace(opts.token) == "" ||
				(resolvedPlatform.ID != platform.Unknown && !resolvedPlatform.Operational) {
				return writeDoctorCredentialOutput(cmd, opts, resolvedPlatform)
			}

			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			publicQuery := url.Values{}
			publicQuery.Set("pageSize", "1")
			var marketResp map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/marketListings", publicQuery, &marketResp); err != nil {
				return fmt.Errorf("check public Market endpoint: %w", err)
			}

			query := url.Values{}
			query.Set("pageSize", "1")
			var probeResp map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/users/me/executables", query, &probeResp); err != nil {
				if isAuthenticationFailure(err) {
					return writeDoctorCredentialOutput(cmd, opts, resolvedPlatform)
				}
				return fmt.Errorf("check authenticated executables endpoint: %w", err)
			}
			healthy := true
			message := "ok"

			versionStatus, versionErr := version.CheckLatest(ctx)
			currentVersion := version.Version
			latestVersion := ""
			currentChannel := version.ReleaseChannel(currentVersion)
			latestChannel := ""
			updateAvailable := false
			upgradeHint := ""
			if versionStatus != nil {
				currentVersion = versionStatus.CurrentVersion
				latestVersion = versionStatus.LatestVersion
				currentChannel = versionStatus.CurrentChannel
				latestChannel = versionStatus.LatestChannel
				updateAvailable = versionStatus.UpdateAvailable
				upgradeHint = versionStatus.UpgradeHint
			}

			if opts.output == "json" {
				payload := map[string]any{
					"server":               opts.server,
					"token_set":            opts.token != "",
					"healthy":              healthy,
					"message":              message,
					"platform":             string(resolvedPlatform.ID),
					"platform_name":        resolvedPlatform.DisplayName,
					"platform_operational": resolvedPlatform.Operational,
					"cli_version":          currentVersion,
					"release_channel":      currentChannel,
					"latest_release":       latestVersion,
					"latest_channel":       latestChannel,
					"update_available":     updateAvailable,
					"upgrade_hint":         upgradeHint,
					"base_usage":           "set LOOMLOOM_SERVER and LOOMLOOM_TOKEN before running template commands",
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(payload)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "server: %s\n", opts.server)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "token: %t\n", opts.token != "")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "platform: %s\n", resolvedPlatform.DisplayName)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "healthy: %t\n", healthy)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "message: %s\n", message)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "cli_version: %s\n", currentVersion)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "release_channel: %s\n", currentChannel)
			if latestVersion != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "latest_release: %s\n", latestVersion)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "latest_channel: %s\n", latestChannel)
			}
			if upgradeHint != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "notice: %s\n", upgradeHint)
			} else if versionErr != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "version check unavailable; skipping release notice")
			}
			return nil
		},
	}
}

func writeDoctorCredentialOutput(cmd *cobra.Command, opts *rootOptions, resolvedPlatform platform.Platform) error {
	action, message := credentialMessageForPlatform(resolvedPlatform)
	if opts.output == "json" {
		payload := map[string]any{
			"server":               opts.server,
			"token_set":            strings.TrimSpace(opts.token) != "",
			"healthy":              false,
			"message":              "credential action required",
			"platform":             string(resolvedPlatform.ID),
			"platform_name":        resolvedPlatform.DisplayName,
			"platform_operational": resolvedPlatform.Operational,
			"credential_action":    action,
			"credential_message":   message,
			"base_usage":           "set LOOMLOOM_SERVER and LOOMLOOM_TOKEN before running template commands",
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), message)
	return err
}

func isAuthenticationFailure(err error) bool {
	var requestErr client.RequestError
	if !errors.As(err, &requestErr) {
		return false
	}
	return requestErr.StatusCode == http.StatusUnauthorized || requestErr.StatusCode == http.StatusForbidden
}
