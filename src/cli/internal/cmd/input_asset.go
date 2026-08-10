package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type uploadInputAssetRequest struct {
	Filename    string `json:"filename"`
	Content     []byte `json:"content"`
	ContentType string `json:"contentType,omitempty"`
}

type uploadInputAssetResponse struct {
	InputAssetID string    `json:"inputAssetId"`
	Filename     string    `json:"filename"`
	MimeType     string    `json:"mimeType"`
	SizeBytes    flexInt64 `json:"sizeBytes"`
	UploadedAt   flexInt64 `json:"uploadedAt"`
}

func newInputAssetCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "input-asset",
		Short: "Upload reusable raw input assets such as text or images",
	}

	cmd.AddCommand(
		newInputAssetUploadCmd(opts),
	)
	return cmd
}

func newInputAssetUploadCmd(opts *rootOptions) *cobra.Command {
	var contentType string

	cmd := &cobra.Command{
		Use:   "upload <file>",
		Short: "Upload one local file and get back an input_asset_id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			path := strings.TrimSpace(args[0])
			payload, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read input file: %w", err)
			}

			req := uploadInputAssetRequest{
				Filename:    filepath.Base(path),
				Content:     payload,
				ContentType: normalizeInputAssetContentType(path, contentType),
			}
			opts.debugf("input asset upload: uploading filename=%s size_bytes=%d", req.Filename, len(payload))

			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			var resp uploadInputAssetResponse
			if err := httpClient.PostJSON(ctx, "/inputAssets:upload", req, &resp); err != nil {
				return err
			}
			opts.debugf("input asset upload: completed input_asset_id=%s", resp.InputAssetID)

			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}

			return printInputAssetUpload(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&contentType, "content-type", "", "Optional MIME type override, for example text/plain or image/png")
	return cmd
}

func normalizeInputAssetContentType(path, override string) string {
	override = strings.TrimSpace(override)
	if override != "" {
		return override
	}
	if byExt := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); byExt != "" {
		mediaType, _, err := mime.ParseMediaType(byExt)
		if err == nil && mediaType != "" {
			return mediaType
		}
		return byExt
	}
	return ""
}
