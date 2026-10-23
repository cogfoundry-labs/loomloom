package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/authflow"
	"github.com/cogfoundry-labs/loomloom/src/cli/internal/workflowpreview"
	"github.com/spf13/cobra"
)

var openPreviewBrowser = authflow.OpenBrowser

const defaultViewerBootstrapURL = "https://www.shengsuanyun.com/zh/loomloom/workbench/viewer/v1/bootstrap.js"

func newTemplateSpecPreviewCmd(opts *rootOptions) *cobra.Command {
	var title, viewerURL string
	var noOpen bool
	configuredViewerURL := strings.TrimSpace(os.Getenv("LOOMLOOM_VIEWER_BOOTSTRAP_URL"))
	if configuredViewerURL == "" {
		configuredViewerURL = defaultViewerBootstrapURL
	}
	command := &cobra.Command{Use: "preview <spec.json>", Short: "Watch a local draft using the independently published workflow viewer", Long: `Start a loopback-only page and watch one UTF-8 file (maximum 1 MiB).
The browser loads public JS/CSS from the ShengSuanYun production viewer by default.
Override it with --viewer-url or LOOMLOOM_VIEWER_BOOTSTRAP_URL; the flag takes precedence.
This viewer setting is independent of --server and API credentials.
For Agent use, keep the process alive and return ready.viewUrl immediately;
ready confirms local startup, not browser resource loading or rendering.
The CLI does not download UI resources. Draft text and session credentials stay
on this computer; no API key, login, upload or remote contract lookup is used.
Keep this process running while editing. Ctrl+C / SIGTERM closes the local
service. --output json emits NDJSON events. Use --no-open for automation.`, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] == "-" || strings.TrimSpace(args[0]) == "" {
			return fmt.Errorf("preview requires a file path, not stdin")
		}
		path, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		enc := json.NewEncoder(cmd.OutOrStdout())
		return workflowpreview.Run(ctx, workflowpreview.Options{File: path, Title: title, ViewerURL: viewerURL}, func(event workflowpreview.Event) error {
			if opts.output == "json" {
				if err := enc.Encode(event); err != nil {
					return err
				}
			} else {
				var text string
				switch event.Type {
				case "ready":
					text = "viewUrl: " + event.ViewURL + "\nLocal draft only. Keep this process running; preview does not save or run a template."
				case "synced":
					text = "synced sequence=" + event.Sequence
				case "waiting":
					text = "waiting: " + event.Code
				case "ended":
					text = "local preview ended"
				}
				_, err := fmt.Fprintln(cmd.OutOrStdout(), text)
				if err != nil {
					return err
				}
			}
			if event.Type == "ready" && !noOpen {
				if err := openPreviewBrowser(event.ViewURL); err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), "Could not open the browser; use the local viewUrl above.")
				}
			}
			return nil
		})
	}}
	command.Flags().StringVar(&title, "title", "", "Optional preview title (up to 256 UTF-8 bytes)")
	command.Flags().StringVar(&viewerURL, "viewer-url", configuredViewerURL, "HTTPS viewer/v1/bootstrap.js URL (overrides LOOMLOOM_VIEWER_BOOTSTRAP_URL and the production default)")
	command.Flags().BoolVar(&noOpen, "no-open", false, "Print the local link without opening a browser")
	return command
}
