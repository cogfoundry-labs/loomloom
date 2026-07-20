package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/client"
	"github.com/Cogfoundry-ai/loomloom/cli/internal/platform"
	"github.com/Cogfoundry-ai/loomloom/cli/internal/version"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	server                    string
	token                     string
	enforceServerVerification bool
	timeout                   time.Duration
	output                    string
	verbose                   bool
	logWriter                 io.Writer
}

func NewRootCmd() *cobra.Command {
	server := configuredServer()
	opts := &rootOptions{
		server:                    server,
		token:                     "",
		enforceServerVerification: true,
		timeout:                   30 * time.Second,
		output:                    "text",
		verbose:                   envBool("LOOMLOOM_VERBOSE"),
	}

	cmd := &cobra.Command{
		Use:           "loomloom",
		Short:         "Developer CLI for LoomLoom workflows",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			opts.logWriter = cmd.ErrOrStderr()
			opts.output = strings.ToLower(strings.TrimSpace(opts.output))
			if opts.output != "text" && opts.output != "json" {
				return fmt.Errorf("unsupported output format %q; use text or json", opts.output)
			}
			if strings.TrimSpace(opts.server) != "" && cmd.Name() != "doctor" {
				normalized, err := platform.NormalizeServer(opts.server)
				if err != nil {
					return err
				}
				opts.server = normalized
			}
			if !flagChanged(cmd, "token") {
				opts.token, _ = configuredToken(opts.server)
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&opts.server, "server", "s", opts.server, "LoomLoom base URL or host")
	if serverFlag := cmd.PersistentFlags().Lookup("server"); serverFlag != nil {
		serverFlag.DefValue = ""
	}
	cmd.PersistentFlags().StringVarP(&opts.token, "token", "t", opts.token, "Bearer token")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", opts.timeout, "HTTP timeout")
	cmd.PersistentFlags().StringVarP(&opts.output, "output", "o", opts.output, "Output format: text|json")
	cmd.PersistentFlags().BoolVarP(&opts.verbose, "verbose", "v", opts.verbose, "Write debug logs to stderr")
	if tokenFlag := cmd.PersistentFlags().Lookup("token"); tokenFlag != nil {
		tokenFlag.DefValue = ""
	}

	cmd.AddCommand(
		newDoctorCmd(opts),
		newModelCmd(opts),
		newAssetCmd(opts),
		newMarketCmd(opts),
		newListingCmd(opts),
		newCreatorCmd(opts),
		newUsageCmd(opts),
		newInputAssetCmd(opts),
		newOrchestrationInputCmd(opts),
		newRunCmd(opts),
		newTemplateCmd(opts),
		newTemplateSpecCmd(opts),
		newSkillCmd(opts),
		newArtifactCmd(opts),
		newServerCmd(opts),
	)
	return cmd
}

func newHTTPClient(opts *rootOptions) (*client.Client, error) {
	return newHTTPClientWithVerification(opts, false)
}

func newHTTPClientForDoctor(opts *rootOptions) (*client.Client, error) {
	return newHTTPClientWithVerification(opts, true)
}

func newHTTPClientWithVerification(opts *rootOptions, allowUnverified bool) (*client.Client, error) {
	if err := validateTokenPlatform(opts, allowUnverified); err != nil {
		return nil, err
	}
	return client.New(client.Config{
		BaseURL:   opts.server,
		Token:     opts.token,
		Timeout:   opts.timeout,
		Verbose:   opts.verbose,
		LogWriter: opts.logWriter,
	})
}

func configuredServer() string {
	state := platform.LoadState()
	if active, ok := state.ActiveProfile(); ok {
		return active.Server
	}
	path, pathErr := platform.StatePath()
	if pathErr == nil {
		if _, err := os.Stat(path); err == nil {
			return ""
		}
	}
	return strings.TrimSpace(os.Getenv("LOOMLOOM_SERVER"))
}

func configuredToken(server string) (string, string) {
	state := platform.LoadState()
	if profile, ok := state.FindProfile(server); ok && strings.TrimSpace(profile.TokenEnv) != "" {
		if value := strings.TrimSpace(os.Getenv(profile.TokenEnv)); value != "" {
			return value, profile.TokenEnv
		}
		return "", profile.TokenEnv
	}
	if value := strings.TrimSpace(os.Getenv("LOOMLOOM_TOKEN")); value != "" {
		return value, "LOOMLOOM_TOKEN"
	}
	return "", ""
}

func flagChanged(cmd *cobra.Command, name string) bool {
	return cmd.Flags().Changed(name) || cmd.InheritedFlags().Changed(name)
}

func envBool(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func (opts *rootOptions) debugf(format string, args ...any) {
	if !opts.verbose || opts.logWriter == nil {
		return
	}
	_, _ = fmt.Fprintf(opts.logWriter, "[debug] "+format+"\n", args...)
}
