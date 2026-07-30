package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/platform"
	"github.com/spf13/cobra"
)

func newServerCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage verified LoomLoom server profiles",
	}
	cmd.AddCommand(
		newServerListCmd(opts),
		newServerUseCmd(opts),
		newServerRemoveCmd(opts),
	)
	return cmd
}

func newServerListCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List verified LoomLoom server profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			state := platform.LoadState()
			profiles := append([]platform.Profile(nil), state.Servers...)
			sort.SliceStable(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
			if opts.output == "json" {
				items := make([]map[string]any, 0, len(profiles))
				for _, profile := range profiles {
					items = append(items, profileOutput(profile, profile.Name == state.ActiveServer))
				}
				return writeJSON(cmd, map[string]any{
					"active_server": state.ActiveServer,
					"servers":       items,
				})
			}
			if len(profiles) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "No verified LoomLoom servers configured.")
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "ACTIVE\tNAME\tPLATFORM\tTOKEN SET\tSERVER")
			for _, profile := range profiles {
				active := ""
				if profile.Name == state.ActiveServer {
					active = "*"
				}
				_, _ = fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s\t%s\t%s\t%t\t%s\n",
					active,
					profile.Name,
					profile.Platform,
					profileTokenSet(profile),
					profile.Server,
				)
			}
			return nil
		},
	}
}

func newServerUseCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name-or-server>",
		Short: "Set the active LoomLoom server profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			state := platform.LoadState()
			profile, err := state.Use(args[0])
			if err != nil {
				return err
			}
			if err := platform.SaveState(state); err != nil {
				return fmt.Errorf("save active LoomLoom server: %w", err)
			}
			tokenSet := profileTokenSet(profile)
			if opts.output == "json" {
				return writeJSON(cmd, profileOutput(profile, true))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "active server: %s (%s)\n", profile.Name, profile.Server)
			if !tokenSet {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "token not set: configure %s before authenticated commands\n", profile.TokenEnv)
			}
			return nil
		},
	}
}

func newServerRemoveCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name-or-server>",
		Short: "Remove a saved LoomLoom server profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			state := platform.LoadState()
			profile, err := state.Remove(args[0])
			if err != nil {
				return err
			}
			if err := platform.SaveState(state); err != nil {
				return fmt.Errorf("remove LoomLoom server profile: %w", err)
			}
			if opts.output == "json" {
				return writeJSON(cmd, map[string]any{
					"removed":       profile.Name,
					"server":        profile.Server,
					"token_env":     profile.TokenEnv,
					"active_server": state.ActiveServer,
				})
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "removed server: %s (%s)\n", profile.Name, profile.Server)
			if strings.TrimSpace(profile.Token) != "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "saved login credential removed")
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "token environment variable was not removed: %s\n", profile.TokenEnv)
			return nil
		},
	}
}

func profileOutput(profile platform.Profile, active bool) map[string]any {
	return map[string]any{
		"active":      active,
		"name":        profile.Name,
		"platform":    string(profile.Platform),
		"server":      profile.Server,
		"token_env":   profile.TokenEnv,
		"token_set":   profileTokenSet(profile),
		"verified_at": profile.VerifiedAt,
	}
}

func profileTokenSet(profile platform.Profile) bool {
	return strings.TrimSpace(os.Getenv(profile.TokenEnv)) != "" || strings.TrimSpace(profile.Token) != ""
}
