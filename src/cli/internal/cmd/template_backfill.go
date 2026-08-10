package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newTemplateBackfillResultsCmd(opts *rootOptions) *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "backfill-results <run-id> <xlsx-path>",
		Short: "Backfill one official template workbook with run results",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := strings.TrimSpace(args[0])
			workbookPath := strings.TrimSpace(args[1])

			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			resp, err := httpClient.GetBinary(ctx, "/users/me/runs/"+runID+"/resultWorkbook")
			if err != nil {
				return err
			}
			targetPath, err := resolveBackfillOutputPath(outputPath, workbookPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(targetPath, resp.Body, 0o644); err != nil {
				return fmt.Errorf("write result workbook: %w", err)
			}

			result := map[string]any{
				"runId":       runID,
				"inputFile":   workbookPath,
				"outputFile":  targetPath,
				"size":        len(resp.Body),
				"contentType": resp.ContentType,
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"run_id\t%s\ninput_file\t%s\noutput_file\t%s\nsize\t%d\n",
				runID,
				workbookPath,
				targetPath,
				len(resp.Body),
			)
			return err
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output-file", "f", "", "Optional output .xlsx path; defaults to overwriting the input workbook")
	return cmd
}

func resolveBackfillOutputPath(outputPath string, workbookPath string) (string, error) {
	if strings.TrimSpace(outputPath) == "" {
		return filepath.Abs(workbookPath)
	}
	defaultName := strings.TrimSuffix(filepath.Base(workbookPath), filepath.Ext(workbookPath)) + ".result.xlsx"
	return resolveFilePath(outputPath, defaultName)
}
