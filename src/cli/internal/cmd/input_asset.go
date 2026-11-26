package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/client"
	"github.com/spf13/cobra"
)

const defaultInputAssetUploadTimeout = 65 * time.Minute

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
		Short: "Upload reusable raw input assets such as text, images, or videos",
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
			uploadOpts := *opts
			uploadOpts.timeout = resolveInputAssetUploadTimeout(cmd, opts.timeout)

			httpClient, err := newHTTPClient(&uploadOpts)
			if err != nil {
				return err
			}
			path := strings.TrimSpace(args[0])
			filename := filepath.Base(path)
			resolvedContentType, err := detectInputAssetContentType(path, contentType)
			if err != nil {
				return err
			}
			opts.debugf("input asset upload: streaming filename=%s content_type=%s", filename, resolvedContentType)

			ctx, cancel := context.WithTimeout(cmd.Context(), uploadOpts.timeout)
			defer cancel()

			var resp uploadInputAssetResponse
			if err := httpClient.PostStreamFile(ctx, "/inputAssets:uploadStream", path, client.StreamUploadOptions{
				Filename:    filename,
				ContentType: resolvedContentType,
			}, &resp); err != nil {
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

	cmd.Flags().StringVar(&contentType, "content-type", "", "Optional MIME type override, for example text/plain, image/png, or video/mp4")
	return cmd
}

func resolveInputAssetUploadTimeout(cmd *cobra.Command, configured time.Duration) time.Duration {
	if !flagChanged(cmd, "timeout") && configured == defaultHTTPTimeout {
		return defaultInputAssetUploadTimeout
	}
	return configured
}

var fallbackInputAssetMIMETypes = map[string]string{
	".mp4":  "video/mp4",
	".mov":  "video/quicktime",
	".m4v":  "video/x-m4v",
	".webm": "video/webm",
	".mkv":  "video/x-matroska",
	".avi":  "video/x-msvideo",
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".m4a":  "audio/mp4",
	".txt":  "text/plain",
	".md":   "text/markdown",
	".csv":  "text/csv",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
}

func detectInputAssetContentType(path, override string) (string, error) {
	if resolved := normalizeInputAssetContentType(path, override); resolved != "" {
		return resolved, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read input file: %w", err)
	}
	defer func() { _ = file.Close() }()

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read input file: %w", err)
	}
	if n > 0 {
		if mediaType, _, parseErr := mime.ParseMediaType(http.DetectContentType(buf[:n])); parseErr == nil && mediaType != "" {
			return mediaType, nil
		}
	}
	return "application/octet-stream", nil
}

func normalizeInputAssetContentType(path, override string) string {
	override = strings.TrimSpace(override)
	if override != "" {
		if mediaType, _, err := mime.ParseMediaType(override); err == nil && mediaType != "" {
			return mediaType
		}
		return override
	}
	ext := strings.ToLower(filepath.Ext(path))
	if byExt := mime.TypeByExtension(ext); byExt != "" {
		if mediaType, _, err := mime.ParseMediaType(byExt); err == nil && mediaType != "" {
			return mediaType
		}
		return byExt
	}
	if fallback, ok := fallbackInputAssetMIMETypes[ext]; ok {
		return fallback
	}
	return ""
}
