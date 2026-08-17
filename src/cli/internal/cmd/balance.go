package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newBalanceCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "balance",
		Short: "Show my LoomLoom settled balance snapshot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			var resp userBalanceSnapshotResponse
			if err := httpClient.GetJSON(ctx, "/users/me/balance", &resp); err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}
			currency := strings.TrimSpace(resp.Currency)
			settled, err := formatResponseMoney(resp.SettledMoney, resp.SettledBalance, currency)
			if err != nil {
				return err
			}
			tw := newTabWriter(cmd.OutOrStdout())
			if _, err := fmt.Fprintf(tw, "settled_balance\t%s\navailability\t%s\nfinal_admission\t%s\n", settled, resp.Availability, resp.FinalAdmission); err != nil {
				return err
			}
			return tw.Flush()
		},
	}
}
