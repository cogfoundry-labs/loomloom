package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/viewerassets"
	"github.com/cogfoundry-labs/loomloom/src/cli/internal/workflowpreview"
	"github.com/spf13/cobra"
)

func newTemplateSpecPreviewCmd(opts *rootOptions) *cobra.Command {
	var title string
	command := &cobra.Command{Use: "preview <spec.json>", Short: "Watch a local draft in the bundled workflow viewer", Long: `Start a loopback-only viewer and watch one UTF-8 file (maximum 1 MiB).
The file may not exist yet or may contain incomplete JSON. Keep this process
running while editing. Draft text stays on this computer. Only contract binding
identifiers are sent to the configured API to resolve authoritative ports.
No browser login, Redis, save, publication or workflow execution is involved.
Ctrl+C / SIGTERM closes the local server. Restarting creates a new private local
link. Other devices cannot use it. --output json emits NDJSON events.`, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] == "-" || strings.TrimSpace(args[0]) == "" {
			return fmt.Errorf("preview requires a file path, not stdin")
		}
		path, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		assets, err := viewerassets.Open()
		if err != nil {
			return fmt.Errorf("load bundled viewer: %w", err)
		}
		// Missing remote configuration must not prevent a local draft from opening.
		var resolver workflowpreview.Resolver
		if httpClient, clientErr := newHTTPClient(opts); clientErr == nil {
			resolver = func(ctx context.Context, bindings workflowpreview.Bindings) (json.RawMessage, error) {
				var value json.RawMessage
				err := httpClient.PostJSON(ctx, "/workflowViewerContext", bindings, &value)
				if err != nil {
					return nil, err
				}
				var header struct {
					SchemaVersion string `json:"schemaVersion"`
					Source        string `json:"source"`
				}
				if len(value) > 2*1024*1024 || json.Unmarshal(value, &header) != nil || header.SchemaVersion != "workflow-viewer/v1" || header.Source != "preview" {
					return nil, fmt.Errorf("invalid viewer context")
				}
				return value, nil
			}
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		enc := json.NewEncoder(cmd.OutOrStdout())
		return workflowpreview.Run(ctx, assets, resolver, workflowpreview.Options{File: path, Title: title}, func(event workflowpreview.Event) error {
			if opts.output == "json" {
				return enc.Encode(event)
			}
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
			return err
		})
	}}
	command.Flags().StringVar(&title, "title", "", "Optional preview title (up to 256 UTF-8 bytes)")
	return command
}
