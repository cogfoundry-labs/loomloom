package cmd

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type marketBalanceCheckResult struct {
	Currency         string        `json:"currency"`
	CallFee          moneyResponse `json:"callFee"`
	AvailableBalance moneyResponse `json:"availableBalance"`
	IsSufficient     bool          `json:"isSufficient"`
	Shortfall        moneyResponse `json:"shortfall"`
}

func newMarketCheckBalanceCmd(opts *rootOptions) *cobra.Command {
	var callFee, currency string
	cmd := &cobra.Command{
		Use:   "check-balance",
		Short: "Check available balance against a Market call fee",
		Long: "Check the current balance against effectiveCreatorFee from the latest Market quote. " +
			"Use its amount and currency with --call-fee and --currency. Returns a failure when the balance is insufficient; does not submit a run.",
		Example: "  loomloom market check-balance --call-fee 6.00 --currency CNY",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			feeT, err := parseMoneyAmountT(callFee)
			if err != nil {
				return fmt.Errorf("invalid --call-fee: %w", err)
			}
			currency = strings.ToUpper(strings.TrimSpace(currency))
			if !isCurrencyCode(currency) {
				return fmt.Errorf("invalid --currency %q; use the currency from effectiveCreatorFee", currency)
			}
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			balance, err := fetchUserBalance(ctx, httpClient)
			if err != nil {
				return err
			}
			availableT, err := marketBalanceAmountT(balance, currency)
			if err != nil {
				return fmt.Errorf("cannot check call fee balance: %w", err)
			}
			fee := big.NewInt(feeT)
			shortfall := new(big.Int).Sub(fee, availableT)
			sufficient := shortfall.Sign() <= 0
			if sufficient {
				shortfall.SetInt64(0)
			}
			result := marketBalanceCheckResult{
				Currency:         currency,
				CallFee:          marketBalanceMoney(fee, currency),
				AvailableBalance: marketBalanceMoney(availableT, currency),
				IsSufficient:     sufficient,
				Shortfall:        marketBalanceMoney(shortfall, currency),
			}
			if opts.output == "json" {
				if err := writeIndentedJSON(cmd.OutOrStdout(), result); err != nil {
					return err
				}
			} else {
				tw := newTabWriter(cmd.OutOrStdout())
				if _, err := fmt.Fprintf(tw, "call_fee\t%s %s\navailable_balance\t%s %s\nsufficient\t%t\nshortfall\t%s %s\n",
					currency, result.CallFee.Amount, currency, result.AvailableBalance.Amount,
					sufficient, currency, result.Shortfall.Amount); err != nil {
					return err
				}
				if err := tw.Flush(); err != nil {
					return err
				}
			}
			if !sufficient {
				message := fmt.Sprintf("insufficient balance for call fee: available %s %s, call fee %s %s, shortfall %s %s; recharge before execution",
					currency, result.AvailableBalance.Amount, currency, result.CallFee.Amount, currency, result.Shortfall.Amount)
				return errors.New(message)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&callFee, "call-fee", "", "Total effectiveCreatorFee.amount from the latest quote, in currency units")
	cmd.Flags().StringVar(&currency, "currency", "", "effectiveCreatorFee.currency from the same quote")
	_ = cmd.MarkFlagRequired("call-fee")
	_ = cmd.MarkFlagRequired("currency")
	return cmd
}

var signedBalanceDecimal = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)$`)

func marketBalanceAmountT(resp userBalanceSnapshotResponse, currency string) (*big.Int, error) {
	responseCurrency := strings.ToUpper(strings.TrimSpace(resp.Currency))
	if responseCurrency != "" && responseCurrency != currency {
		return nil, fmt.Errorf("balance currency %s does not match call fee currency %s", responseCurrency, currency)
	}
	if resp.AvailableMoney == nil {
		if resp.AvailableBalance == nil {
			return nil, errors.New("available balance is missing")
		}
		if responseCurrency == "" {
			return nil, errors.New("balance currency is missing")
		}
		return big.NewInt(int64(*resp.AvailableBalance)), nil
	}
	if strings.ToUpper(strings.TrimSpace(resp.AvailableMoney.Currency)) != currency {
		return nil, errors.New("available balance currency does not match call fee currency")
	}
	amount := strings.TrimSpace(resp.AvailableMoney.Amount)
	if !signedBalanceDecimal.MatchString(amount) {
		return nil, errors.New("available balance must be a decimal amount")
	}
	value, ok := new(big.Rat).SetString(amount)
	if !ok {
		return nil, errors.New("invalid available balance")
	}
	value.Mul(value, big.NewRat(10_000_000, 1))
	if !value.IsInt() || !value.Num().IsInt64() {
		return nil, errors.New("available balance is outside the supported range or precision")
	}
	if resp.AvailableBalance != nil && value.Num().Int64() != int64(*resp.AvailableBalance) {
		return nil, errors.New("available balance amount does not match availableBalanceT")
	}
	return new(big.Int).Set(value.Num()), nil
}

func marketBalanceMoney(amountT *big.Int, currency string) moneyResponse {
	value := new(big.Rat).SetFrac(amountT, big.NewInt(10_000_000))
	return moneyResponse{Amount: trimDisplayDecimal(value.FloatString(7)), Currency: currency}
}
