package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseMoneyAmountT(t *testing.T) {
	tests := []struct {
		raw  string
		want int64
	}{
		{raw: "0.5", want: 5_000_000},
		{raw: "1", want: 10_000_000},
		{raw: "0.000001", want: 10},
		{raw: "0.0000001", want: 1},
		{raw: "0.50000000", want: 5_000_000},
		{raw: ".5", want: 5_000_000},
		{raw: "1.", want: 10_000_000},
		{raw: "0.", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := parseMoneyAmountT(tt.raw)
			if err != nil {
				t.Fatalf("parseMoneyAmountT(%q) error=%v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("parseMoneyAmountT(%q)=%d want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseMoneyAmountTRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "", want: "amount is required"},
		{raw: "-0.1", want: "non-negative"},
		{raw: ".", want: "decimal"},
		{raw: "0.00000001", want: "0.0000001"},
		{raw: "1/2", want: "decimal"},
		{raw: "1e-1", want: "decimal"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			_, err := parseMoneyAmountT(tt.raw)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseMoneyAmountT(%q) error=%v want containing %q", tt.raw, err, tt.want)
			}
		})
	}
}

func TestTrimDisplayDecimalRemovesOnlyInsignificantZeros(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"2000.0000000", "2000"},
		{"0.0119350", "0.011935"},
		{"176.3397374", "176.3397374"},
		{"0.0000001", "0.0000001"},
		{"-0.0000000", "0"},
	} {
		if got := trimDisplayDecimal(test.input); got != test.want {
			t.Fatalf("trimDisplayDecimal(%q)=%q want %q", test.input, got, test.want)
		}
	}
}

func TestFormatResponseMoneyPrefersMoneyAndSupportsLegacyFallback(t *testing.T) {
	raw := flexInt64(5_000_000)
	tests := []struct {
		name     string
		money    *moneyResponse
		raw      *flexInt64
		currency string
		want     string
	}{
		{
			name:  "money only",
			money: &moneyResponse{Amount: "0.5000000", Currency: "CNY"},
			want:  "CNY 0.5",
		},
		{
			name:     "money preferred when equivalent raw exists",
			money:    &moneyResponse{Amount: "0.5000000", Currency: "CNY"},
			raw:      &raw,
			currency: "CNY",
			want:     "CNY 0.5",
		},
		{
			name:     "legacy fallback",
			raw:      &raw,
			currency: "CNY",
			want:     "CNY 0.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatResponseMoney(tt.money, tt.raw, tt.currency)
			if err != nil {
				t.Fatalf("formatResponseMoney() error=%v", err)
			}
			if got != tt.want {
				t.Fatalf("formatResponseMoney()=%q want %q", got, tt.want)
			}
		})
	}
}

func TestFormatResponseMoneyRejectsContractMismatch(t *testing.T) {
	raw := flexInt64(5_000_001)
	tests := []struct {
		name     string
		money    *moneyResponse
		raw      *flexInt64
		currency string
		want     string
	}{
		{
			name:  "amount mismatch",
			money: &moneyResponse{Amount: "0.5000000", Currency: "CNY"},
			raw:   &raw,
			want:  "money amount 0.5000000 does not match raw amount 5000001",
		},
		{
			name:     "currency mismatch",
			money:    &moneyResponse{Amount: "0.5000000", Currency: "USD"},
			currency: "CNY",
			want:     "money currency USD does not match response currency CNY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := formatResponseMoney(tt.money, tt.raw, tt.currency)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("formatResponseMoney() error=%v want containing %q", err, tt.want)
			}
		})
	}
}

func TestPrintPrecheckAndRunSummaryUseMoney(t *testing.T) {
	t.Run("precheck", func(t *testing.T) {
		var out bytes.Buffer
		err := printPrecheck(&out, precheckTemplateRowsResponse{
			EstimatedTotalCost: &moneyResponse{Amount: "1.7027920", Currency: "CNY"},
			BalanceCheck: &templateBalanceCheck{
				Currency: "CNY", AvailableBalanceMoney: &moneyResponse{Amount: "2.0000000", Currency: "CNY"},
				IsSufficient: true,
			},
		})
		if err != nil {
			t.Fatalf("printPrecheck() error=%v", err)
		}
		for _, want := range []string{"estimated_cost", "CNY 1.702792", "available_balance", "CNY 2", "sufficient"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("output=%q missing %q", out.String(), want)
			}
		}
	})

	t.Run("run summary", func(t *testing.T) {
		var out bytes.Buffer
		err := printRunSummary(&out, runStatusResponse{
			RunID:      "run-1",
			Status:     "completed",
			ActualCost: &moneyResponse{Amount: "1.7027920", Currency: "CNY"},
		})
		if err != nil {
			t.Fatalf("printRunSummary() error=%v", err)
		}
		if !strings.Contains(out.String(), "CNY 1.702792") {
			t.Fatalf("output=%q want Money amount", out.String())
		}
	})
}
