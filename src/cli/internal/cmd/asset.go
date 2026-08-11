package cmd

import (
	"context"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newAssetCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asset",
		Short: "List executable Product API assets",
	}
	cmd.AddCommand(newAssetListCmd(opts))
	return cmd
}

func newAssetListCmd(opts *rootOptions) *cobra.Command {
	var pageSize int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List executable assets for the current user",
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			query := url.Values{}
			if pageSize > 0 {
				query.Set("pageSize", fmt.Sprintf("%d", pageSize))
			}

			var resp map[string]any
			if err := httpClient.GetProductJSONWithQuery(ctx, "/users/me/executables", query, &resp); err != nil {
				return err
			}
			return writeIndentedJSON(cmd.OutOrStdout(), resp)
		},
	}
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Page size")
	return cmd
}
