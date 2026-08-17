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
			available, err := formatSignedBalanceMoney(resp.AvailableMoney, resp.AvailableBalance, strings.TrimSpace(resp.Currency))
			if err != nil {
				return err
			}
			tw := newTabWriter(cmd.OutOrStdout())
			if _, err := fmt.Fprintf(tw, "available_balance\t%s\n", available); err != nil {
				return err
			}
			return tw.Flush()
		},
	}
}

func formatSignedBalanceMoney(money *moneyResponse, amountT *flexInt64, currency string) (string, error) {
	if money != nil {
		currency := strings.ToUpper(strings.TrimSpace(money.Currency))
		if !isCurrencyCode(currency) {
			return "", fmt.Errorf("invalid money currency %q", money.Currency)
		}
		return currency + " " + strings.TrimSpace(money.Amount), nil
	}
	if amountT == nil {
		return formatMoneyT(0, currency), nil
	}
	return formatMoneyT(int64(*amountT), currency), nil
}
