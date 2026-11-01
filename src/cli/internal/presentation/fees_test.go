package presentation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHideEstimatesPreservesBusinessData(t *testing.T) {
	var response map[string]any
	err := json.Unmarshal([]byte(`{
		"run":{"estimatedCostT":987654321,"market":{"estimatedBuyerPayable":{"amount":"98.7654321","currency":"CNY"},"effectiveCreatorFee":{"amount":"2","currency":"CNY"}}},
		"items":[{"estimatedExecutionCostT":987654321,"actualExecutionCostT":123}],
		"quote":{"estimatedExecutionCost":{"amount":"98.7654321","currency":"CNY"},"listingVersionId":"v1","effectivePaymentMode":"subscription"},
		"precheck":{"estimatedTotalCostT":987654321,"balanceCheck":{"availableBalance":456,"isSufficient":false}},
		"transaction":{"estimatedBuyerPayableT":987654321,"runTransactionId":"rt1"},
		"input":{"estimatedCost":42},"result":{"quote":{"estimatedCost":43}},
		"workflowDefinition":{"estimatedCost":44},"unknownField":"preserved"
	}`), &response)
	if err != nil {
		t.Fatal(err)
	}
	HideEstimates(response)
	encoded, _ := json.Marshal(response)
	for _, hidden := range []string{"987654321", "98.7654321"} {
		if strings.Contains(string(encoded), hidden) {
			t.Fatalf("estimate leaked: %s", encoded)
		}
	}
	for _, kept := range []string{`"actualExecutionCostT":123`, `"availableBalance":456`, `"isSufficient":false`, `"effectiveCreatorFee"`, `"listingVersionId":"v1"`, `"effectivePaymentMode":"subscription"`, `"runTransactionId":"rt1"`, `"estimatedCost":42`, `"estimatedCost":43`, `"estimatedCost":44`, `"unknownField":"preserved"`} {
		if !strings.Contains(string(encoded), kept) {
			t.Fatalf("lost %s: %s", kept, encoded)
		}
	}
}

func TestErrorBodyHidesAmountsAndPreservesFailureDetails(t *testing.T) {
	tests := []struct {
		name, body   string
		hidden, kept []string
	}{
		{"structured estimate", `{"code":"estimate_failed","error":"service unavailable","estimatedTotalCostT":881230000,"details":{"estimatedCost":{"amount":"88.123","currency":"CNY"},"stepId":"step-42"}}`, []string{"881230000", "88.123"}, []string{"estimate_failed", "service unavailable", "step-42"}},
		{"labelled amount", `{"error":"step 42: estimated cost CNY 88.123 exceeds limit"}`, []string{"88.123"}, []string{"step 42", "exceeds limit"}},
		{"camel amount", `{"error":"estimatedTotalCostT=881230000; HTTP 503"}`, []string{"881230000"}, []string{"HTTP 503"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorBody(tt.body)
			for _, hidden := range tt.hidden {
				if strings.Contains(got, hidden) {
					t.Fatalf("leaked %q: %s", hidden, got)
				}
			}
			for _, kept := range tt.kept {
				if !strings.Contains(got, kept) {
					t.Fatalf("lost %q: %s", kept, got)
				}
			}
		})
	}
	for _, body := range []string{
		`{ "code": "insufficient_balance", "error": "Insufficient balance to continue this execution. Please recharge and retry.", "requestId": "req-42" }`,
		`{"code":"insufficient_balance","message":"Please recharge and retry.","traceId":"trace-42"}`,
		`{"error":"insufficient balance: current 1.0000 CNY, required 88.1230 CNY"}`,
		`submit: insufficient balance: current 1.0000 CNY, required 88.1230 CNY`,
		`{ "error": "row 12: image returned HTTP 404", "code": "invalid_input" }`,
		`{"error":"estimate step 42: model pricing unavailable","code":"pricing_unavailable"}`,
		`{"error":"permission denied","code":"forbidden"}`,
		`{"error":"call fee changed from 2 CNY to 3 CNY","code":"price_changed"}`,
	} {
		if got := ErrorBody(body); got != body {
			t.Fatalf("unrelated error changed: %s -> %s", body, got)
		}
	}
}
