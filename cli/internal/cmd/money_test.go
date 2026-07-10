package cmd

import (
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
