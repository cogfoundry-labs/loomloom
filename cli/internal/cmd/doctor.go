package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/client"
	"github.com/Cogfoundry-ai/loomloom/cli/internal/platform"
	"github.com/Cogfoundry-ai/loomloom/cli/internal/version"
	"github.com/spf13/cobra"
)

func newDoctorCmd(opts *rootOptions) *cobra.Command {
	var requestedName string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check LoomLoom server reachability and token wiring",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.server) != "" {
				normalized, err := platform.NormalizeServer(opts.server)
				if err != nil {
					return writeDoctorFailure(cmd, opts, platform.UnknownPlatform(), "fix_server", "invalid server URL", err.Error(), false)
				}
				opts.server = normalized
			}
			resolvedPlatform, err := resolvePlatform(opts)
			if err != nil {
				return err
			}
			if strings.TrimSpace(opts.server) == "" {
				return writeDoctorCredentialOutput(cmd, opts, resolvedPlatform, "choose_server", false)
			}
			if strings.TrimSpace(opts.token) == "" {
				return writeDoctorCredentialOutput(cmd, opts, resolvedPlatform, "configure_token", false)
			}

			httpClient, err := newHTTPClientForDoctor(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			publicQuery := url.Values{}
			publicQuery.Set("pageSize", "1")
			var marketResp map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/marketListings", publicQuery, &marketResp); err != nil {
				return writeDoctorFailure(cmd, opts, resolvedPlatform, "fix_server", "server probe failed", safeDoctorErrorDetail(err), false)
			}

			query := url.Values{}
			query.Set("pageSize", "1")
			var probeResp map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/users/me/executables", query, &probeResp); err != nil {
				if isAuthenticationFailure(err) {
					return writeDoctorFailure(
						cmd,
						opts,
						resolvedPlatform,
						"replace_token",
						"credential action required",
						tokenAuthenticationFailureMessage(),
						false,
					)
				}
				return writeDoctorFailure(cmd, opts, resolvedPlatform, "fix_server", "authenticated probe failed", safeDoctorErrorDetail(err), false)
			}

			state := platform.LoadState()
			profile, err := state.UpsertVerified(opts.server, resolvedPlatform.ID, requestedName, time.Now())
			if err != nil {
				return err
			}
			if err := platform.SaveState(state); err != nil {
				return fmt.Errorf("save LoomLoom server profile: %w", err)
			}

			nextAction := "persist_token"
			if stored := strings.TrimSpace(os.Getenv(profile.TokenEnv)); stored != "" && stored == strings.TrimSpace(opts.token) {
				nextAction = "none"
			}

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

			payload := map[string]any{
				"server":               opts.server,
				"profile":              profile.Name,
				"token_env":            profile.TokenEnv,
				"token_set":            true,
				"token_valid":          true,
				"healthy":              true,
				"message":              "ok",
				"platform":             string(resolvedPlatform.ID),
				"platform_name":        resolvedPlatform.DisplayName,
				"platform_preset":      isPlatformPreset(resolvedPlatform),
				"platform_operational": resolvedPlatform.Operational,
				"credential_action":    "none",
				"credential_message":   "",
				"next_action":          nextAction,
				"cli_version":          currentVersion,
				"release_channel":      currentChannel,
				"latest_release":       latestVersion,
				"latest_channel":       latestChannel,
				"update_available":     updateAvailable,
				"upgrade_hint":         upgradeHint,
			}
			if opts.output == "json" {
				return writeJSON(cmd, payload)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "server: %s\n", opts.server)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "profile: %s\n", profile.Name)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "platform: %s\n", resolvedPlatform.DisplayName)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "token_set: true")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "token_valid: true")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "healthy: true")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "token_env: %s\n", profile.TokenEnv)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "next_action: %s\n", nextAction)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "cli_version: %s\n", currentVersion)
			if upgradeHint != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "notice: %s\n", upgradeHint)
			} else if versionErr != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "version check unavailable; skipping release notice")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&requestedName, "name", "", "Optional saved server profile name")
	return cmd
}

func writeDoctorCredentialOutput(
	cmd *cobra.Command,
	opts *rootOptions,
	resolvedPlatform platform.Platform,
	nextAction string,
	tokenValid bool,
) error {
	action, message := credentialMessageForPlatform(resolvedPlatform)
	payload := doctorBasePayload(opts, resolvedPlatform)
	payload["healthy"] = false
	payload["token_valid"] = tokenValid
	payload["message"] = "credential action required"
	payload["credential_action"] = action
	payload["credential_message"] = message
	payload["next_action"] = nextAction
	if opts.output == "json" {
		return writeJSON(cmd, payload)
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), message)
	return err
}

func writeDoctorFailure(
	cmd *cobra.Command,
	opts *rootOptions,
	resolvedPlatform platform.Platform,
	nextAction string,
	message string,
	detail string,
	tokenValid bool,
) error {
	payload := doctorBasePayload(opts, resolvedPlatform)
	payload["healthy"] = false
	payload["token_valid"] = tokenValid
	payload["message"] = message
	payload["credential_action"] = nextAction
	payload["credential_message"] = detail
	payload["next_action"] = nextAction
	if opts.output == "json" {
		return writeJSON(cmd, payload)
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "healthy: false\nmessage: %s\nnext_action: %s\ndetail: %s\n", message, nextAction, detail)
	return err
}

func doctorBasePayload(opts *rootOptions, resolvedPlatform platform.Platform) map[string]any {
	payload := map[string]any{
		"server":               opts.server,
		"token_set":            strings.TrimSpace(opts.token) != "",
		"token_valid":          false,
		"platform":             string(resolvedPlatform.ID),
		"platform_name":        resolvedPlatform.DisplayName,
		"platform_preset":      isPlatformPreset(resolvedPlatform),
		"platform_operational": resolvedPlatform.Operational,
	}
	state := platform.LoadState()
	if profile, ok := state.FindProfile(opts.server); ok {
		payload["profile"] = profile.Name
		payload["token_env"] = profile.TokenEnv
	}
	return payload
}

func isPlatformPreset(p platform.Platform) bool {
	return p.ID == platform.ShengSuanYun || p.ID == platform.CogFoundry
}

func writeJSON(cmd *cobra.Command, payload any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func safeDoctorErrorDetail(err error) string {
	var requestErr client.RequestError
	if errors.As(err, &requestErr) {
		return fmt.Sprintf("server returned HTTP %d", requestErr.StatusCode)
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "request timed out"
	case errors.Is(err, context.Canceled):
		return "request canceled"
	default:
		return "request failed; check Server connectivity"
	}
}

func isAuthenticationFailure(err error) bool {
	var requestErr client.RequestError
	if !errors.As(err, &requestErr) {
		return false
	}
	return requestErr.StatusCode == http.StatusUnauthorized || requestErr.StatusCode == http.StatusForbidden
}
