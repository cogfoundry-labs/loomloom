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

type uploadOrchestrationInputRequest struct {
	Filename string `json:"filename"`
	Content  []byte `json:"content"`
}

type uploadOrchestrationInputResponse struct {
	InputFileID string    `json:"inputFileId"`
	Filename    string    `json:"filename"`
	RowCount    int32     `json:"rowCount"`
	UploadedAt  flexInt64 `json:"uploadedAt"`
}

func newOrchestrationInputCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orchestration-input",
		Short: "Upload JSONL execution inputs for private template runs",
	}
	cmd.AddCommand(newOrchestrationInputUploadCmd(opts))
	return cmd
}

func newOrchestrationInputUploadCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "upload <file.jsonl>",
		Short: "Upload flat JSONL and get the input_file_id required by template-spec run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := strings.TrimSpace(args[0])
			if !strings.EqualFold(filepath.Ext(path), ".jsonl") {
				return fmt.Errorf("orchestration input must be a .jsonl file")
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read orchestration input file: %w", err)
			}

			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			req := uploadOrchestrationInputRequest{
				Filename: filepath.Base(path),
				Content:  content,
			}
			opts.debugf("orchestration input upload: uploading filename=%s size_bytes=%d", req.Filename, len(content))
			var resp uploadOrchestrationInputResponse
			if err := httpClient.PostProductJSON(ctx, "/orchestrationInputs:upload", req, &resp); err != nil {
				return err
			}
			if strings.TrimSpace(resp.InputFileID) == "" {
				return fmt.Errorf("orchestration input upload returned an empty inputFileId")
			}
			opts.debugf("orchestration input upload: completed input_file_id=%s row_count=%d", resp.InputFileID, resp.RowCount)

			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}
			return printOrchestrationInputUpload(cmd.OutOrStdout(), resp)
		},
	}
}
