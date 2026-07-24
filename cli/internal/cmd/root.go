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
	server    string
	token     string
	timeout   time.Duration
	output    string
	verbose   bool
	logWriter io.Writer
}

func NewRootCmd() *cobra.Command {
	opts := &rootOptions{
		server:  configuredServer(),
		token:   configuredToken(),
		timeout: 30 * time.Second,
		output:  "text",
		verbose: envBool("LOOMLOOM_VERBOSE"),
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
		newLoginCmd(opts),
		newLogoutCmd(opts),
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
	)
	return cmd
}

func newHTTPClient(opts *rootOptions) (*client.Client, error) {
	if err := validateTokenPlatform(opts); err != nil {
		return nil, err
	}
	return client.New(client.Config{
		BaseURL:   opts.server,
		Token:     opts.token,
		Timeout:   opts.timeout,
		Verbose:   opts.verbose,
		LogWriter: opts.logWriter,
		OnSuccess: func(meta client.SuccessMeta) {
			if isAuthenticatedProductPath(meta) {
				maybePersistVerifiedPlatform(opts, true)
			}
		},
	})
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func configuredServer() string {
	if value := strings.TrimSpace(envOrDefault("LOOMLOOM_SERVER", os.Getenv("BATCHJOB_SERVER"))); value != "" {
		return value
	}
	return strings.TrimSpace(platform.LoadState().Server)
}

func configuredToken() string {
	if value := envOrDefault("LOOMLOOM_TOKEN", os.Getenv("BATCHJOB_TOKEN")); value != "" {
		return value
	}
	return strings.TrimSpace(platform.LoadState().Token)
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
