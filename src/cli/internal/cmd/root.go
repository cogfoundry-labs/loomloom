package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/client"
	"github.com/cogfoundry-labs/loomloom/src/cli/internal/platform"
	"github.com/cogfoundry-labs/loomloom/src/cli/internal/version"
	"github.com/spf13/cobra"
)

const defaultHTTPTimeout = 30 * time.Second

var checkStableUpdateNotice = version.CachedStableUpdateNotice
var refreshStableUpdateCache = version.RefreshStableUpdateCache

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
		timeout:                   defaultHTTPTimeout,
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
			if opts.timeout <= 0 {
				return fmt.Errorf("--timeout must be greater than 0")
			}
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
			if cmd.Name() != "login" && cmd.Name() != "logout" && !flagChanged(cmd, "token") {
				var err error
				opts.token, _, err = configuredToken(opts.server)
				if err != nil {
					return err
				}
			}
			writeStableUpdateNotice(cmd, opts)
			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&opts.server, "server", "s", opts.server, "LoomLoom base URL or host")
	if serverFlag := cmd.PersistentFlags().Lookup("server"); serverFlag != nil {
		serverFlag.DefValue = ""
	}
	cmd.PersistentFlags().StringVarP(&opts.token, "token", "t", opts.token, "Bearer token")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", opts.timeout, "HTTP request timeout")
	cmd.PersistentFlags().StringVarP(&opts.output, "output", "o", opts.output, "Output format: text|json")
	cmd.PersistentFlags().BoolVarP(&opts.verbose, "verbose", "v", opts.verbose, "Write debug logs to stderr")
	if tokenFlag := cmd.PersistentFlags().Lookup("token"); tokenFlag != nil {
		tokenFlag.DefValue = ""
	}

	cmd.AddCommand(
		newDoctorCmd(opts),
		newLoginCmd(opts),
		newLogoutCmd(opts),
		newModelCmd(opts),
		newCapabilityCmd(opts),
		newBalanceCmd(opts),
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

func writeStableUpdateNotice(cmd *cobra.Command, opts *rootOptions) {
	if opts.output != "text" || cmd.Name() == "doctor" || envBool("LOOMLOOM_NO_UPDATE_CHECK") {
		return
	}
	if notice := checkStableUpdateNotice(); notice != "" {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "notice: %s\n", notice)
	}
	go refreshStableUpdateCache(cmd.Context())
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

func configuredToken(server string) (string, string, error) {
	state := platform.LoadState()
	if profile, ok := state.FindProfile(server); ok {
		if strings.TrimSpace(profile.TokenEnv) != "" {
			value := strings.TrimSpace(os.Getenv(profile.TokenEnv))
			if value != "" && profile.TokenEnv == "LOOMLOOM_TOKEN" {
				if err := validateGlobalTokenServer(server); err != nil {
					return "", "", err
				}
			}
			if value != "" {
				return value, profile.TokenEnv, nil
			}
		}
		if value := strings.TrimSpace(profile.Token); value != "" {
			return value, "saved", nil
		}
		return "", profile.TokenEnv, nil
	}
	if value := strings.TrimSpace(os.Getenv("LOOMLOOM_TOKEN")); value != "" {
		if err := validateGlobalTokenServer(server); err != nil {
			return "", "", err
		}
		return value, "LOOMLOOM_TOKEN", nil
	}
	return "", "", nil
}

func validateGlobalTokenServer(server string) error {
	selected, err := platform.NormalizeServer(server)
	if err != nil {
		return fmt.Errorf("cannot bind LOOMLOOM_TOKEN without a valid Server; pass both --server and --token")
	}
	environmentServer := strings.TrimSpace(os.Getenv("LOOMLOOM_SERVER"))
	if environmentServer == "" {
		state := platform.LoadState()
		if profile, ok := state.FindProfile(selected); ok && profile.TokenEnv == "LOOMLOOM_TOKEN" {
			return nil
		}
		return fmt.Errorf("LOOMLOOM_TOKEN is not bound to the selected Server; pass both --server and --token")
	}
	normalizedEnvironment, err := platform.NormalizeServer(environmentServer)
	if err != nil {
		return fmt.Errorf("LOOMLOOM_SERVER is invalid; pass both --server and --token")
	}
	if normalizedEnvironment != selected {
		return fmt.Errorf("LOOMLOOM_SERVER conflicts with the selected Server; pass both --server and --token to use a different Server")
	}
	return nil
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
