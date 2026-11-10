package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUsageListTextShowsFormattedAmounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items":[{
				"runTransactionId":"rt-1",
				"listingId":"listing-1",
				"skillName":"Writer",
				"taskFixedFeeT":5000000,
				"taskFixedFee":{"amount":"0.5000000","currency":"CNY"},
				"estimatedExecutionCostT":69300,
				"estimatedBuyerPayableT":5069300,
				"actualExecutionCostT":52000,
				"finalBuyerPayableT":5052000,
				"finalBuyerPayable":{"amount":"0.5052000","currency":"CNY"},
				"currency":"CNY",
				"transactionStatus":"settled"
			}],
			"totalCount":1
		}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newUsageListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("usage list command error = %v", err)
	}
	assertContainsAll(t, out.String(),
		"CNY 0.5",
		"CNY 0.5052",
		"rt-1",
		"listing-1",
		"settled",
	)
	assertContainsNone(t, out.String(), "task_fixed_fee_t", "final_payable_t")
	if strings.Contains(out.String(), "{") {
		t.Fatalf("output=%s must not be raw JSON", out.String())
	}
}

func TestUsageListTextUnknownCurrencyDoesNotInventAmounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"runTransactionId":"rt-1","transactionStatus":"succeeded","actualExecutionCostT":2000000,"finalBuyerPayableT":7000000}],"totalCount":1}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newUsageListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("usage list command error = %v", err)
	}
	assertContainsAll(t, out.String(), "unavailable", "rt-1")
	assertContainsNone(t, out.String(), "CNY", "USD", "7000000", "2000000")
}

func TestUsageListRejectsMoneyAndRawMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{
			"runTransactionId":"rt-1",
			"transactionStatus":"succeeded",
			"finalBuyerPayableT":5000001,
			"finalBuyerPayable":{"amount":"0.5000000","currency":"CNY"},
			"currency":"CNY"
		}],"totalCount":1}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newUsageListCmd(opts)

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "money amount 0.5000000 does not match raw amount 5000001") {
		t.Fatalf("usage list error=%v want Money/T contract mismatch", err)
	}
}

func TestUsageListJSONPreservesUnknownFieldsWithoutRawT(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{
			"runTransactionId":"rt-1",
			"transactionStatus":"succeeded",
			"taskFixedFeeT":5000000,
			"taskFixedFee":{"amount":"0.5000000","currency":"CNY"},
			"actualExecutionCostT":2500000,
			"finalBuyerPayableT":7500000,
			"currency":"CNY",
			"recordMetadata":"kept"
		}],"totalCount":1,"newBackendField":"kept"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newUsageListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("usage list command error = %v", err)
	}
	for _, want := range []string{`"actualCreatorFee": {`, `"amount": "0.5"`, `"recordMetadata": "kept"`, `"newBackendField": "kept"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
	assertContainsNone(t, out.String(), "taskFixedFee", "actualExecutionCostT", "finalBuyerPayableT")
}

func TestUsageGetTextShowsFormattedAmounts(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"runTransactionId":"rt-1",
			"runId":"run-1",
			"skillName":"Writer",
			"taskFixedFeeT":5000000,
			"estimatedExecutionCostT":69300,
			"estimatedBuyerPayableT":5069300,
			"actualExecutionCostT":52000,
			"finalBuyerPayableT":5052000,
			"currency":"CNY",
			"transactionStatus":"settled"
		}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newUsageGetCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"rt-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("usage get command error = %v", err)
	}
	if requestedPath != "/loom/v1/users/me/marketUsages/rt-1" {
		t.Fatalf("path=%q want usage get endpoint", requestedPath)
	}
	assertContainsAll(t, out.String(),
		"CNY 0.5",
		"CNY 0.5052",
		"currency",
	)
	assertContainsNone(t, out.String(), "task_fixed_fee_t", "final_payable_t")
}

func TestUsageActualFeesAcrossCommandsAndFormats(t *testing.T) {
	// Rates deliberately differ from charged totals: completed rows and
	// subscriptions must use the final transaction amounts, never the unit rate.
	tests := []struct {
		name, body, status, total, creator, model string
	}{
		{"multi row", `"transactionStatus":"succeeded","currency":"CNY","actualExecutionCostT":43200000,"finalBuyerPayableT":103200000`, "available", "10.32", "6", "4.32"},
		{"subscription", `"transactionStatus":"succeeded","currency":"CNY","actualExecutionCost":{"amount":"4.32","currency":"CNY"},"finalBuyerPayable":{"amount":"4.32","currency":"CNY"}`, "available", "4.32", "0", "4.32"},
		{"partial completion", `"transactionStatus":"succeeded","currency":"CNY","actualExecutionCostT":43200000,"finalBuyerPayableT":63200000`, "available", "6.32", "2", "4.32"},
		{"partial cancellation", `"transactionStatus":"cancelled","currency":"CNY","actualExecutionCostT":43200000,"finalBuyerPayableT":63200000`, "available", "6.32", "2", "4.32"},
		{"failed released", `"transactionStatus":"failed","currency":"CNY","actualExecutionCostT":0,"finalBuyerPayableT":0`, "available", "0", "0", "0"},
		{"cancelled released", `"transactionStatus":"cancelled","currency":"CNY","actualExecutionCostT":0,"finalBuyerPayableT":0`, "available", "0", "0", "0"},
		{"running placeholder", `"transactionStatus":"running","currency":"CNY","actualExecutionCostT":0,"finalBuyerPayableT":0`, "pending", "", "", ""},
		{"run finished transaction pending", `"status":"completed","transactionStatus":"review_needed","currency":"CNY","actualExecutionCostT":0,"finalBuyerPayableT":0`, "pending", "", "", ""},
		{"unknown status", `"transactionStatus":"new_backend_state","currency":"CNY","actualExecutionCostT":0,"finalBuyerPayableT":0`, "pending", "", "", ""},
		{"missing amounts", `"transactionStatus":"succeeded","currency":"CNY"`, "unavailable", "", "", ""},
		{"missing model fee", `"transactionStatus":"succeeded","currency":"CNY","finalBuyerPayableT":103200000`, "unavailable", "10.32", "", ""},
		{"unknown currency", `"transactionStatus":"succeeded","actualExecutionCostT":43200000,"finalBuyerPayableT":103200000`, "unavailable", "", "", ""},
		{"exact large amounts", `"transactionStatus":"succeeded","currency":"CNY","actualExecutionCostT":9007199254740992,"finalBuyerPayableT":9007199254740993`, "available", "900719925.4740993", "0.0000001", "900719925.4740992"},
	}
	for _, tt := range tests {
		for _, list := range []bool{false, true} {
			for _, output := range []string{"text", "json"} {
				t.Run(fmt.Sprintf("%s/list=%t/%s", tt.name, list, output), func(t *testing.T) {
					record := `{"runTransactionId":"rt-1","taskFixedFeeT":999000000,"metadata":"preserved",` + hiddenEstimateFields + `,` + tt.body + `}`
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						if list {
							fmt.Fprintf(w, `{"items":[%s],"totalCount":1}`, record)
						} else {
							fmt.Fprint(w, record)
						}
					}))
					defer server.Close()
					opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: output}
					cmd := newUsageGetCmd(opts)
					cmd.SetArgs([]string{"rt-1"})
					if list {
						cmd = newUsageListCmd(opts)
					}
					var out bytes.Buffer
					cmd.SetOut(&out)
					if err := cmd.Execute(); err != nil {
						t.Fatal(err)
					}
					assertContainsNone(t, out.String(), "estimated", "taskFixedFee", "task_fixed_fee", "98.7654321", "999000000")
					if output == "json" {
						var result map[string]any
						if err := json.Unmarshal(out.Bytes(), &result); err != nil {
							t.Fatal(err)
						}
						if list {
							result = result["items"].([]any)[0].(map[string]any)
						}
						if result["feeStatus"] != tt.status || result["metadata"] != "preserved" {
							t.Fatalf("unexpected fee status or lost metadata: %v", result)
						}
						for key, want := range map[string]string{"finalBuyerPayable": tt.total, "actualCreatorFee": tt.creator, "actualExecutionCost": tt.model} {
							value, exists := result[key]
							if !exists {
								t.Fatalf("missing %s", key)
							}
							if want == "" {
								if value != nil {
									t.Fatalf("%s = %v, want null", key, value)
								}
							} else if money, ok := value.(map[string]any); !ok || money["amount"] != want || money["currency"] != "CNY" {
								t.Fatalf("%s = %v, want CNY %s", key, value, want)
							}
						}
					} else {
						assertContainsAll(t, out.String(), "actual_amount", "creator_call_fee", "model_fee")
						for _, want := range []string{tt.total, tt.creator, tt.model} {
							if want != "" {
								assertContainsAll(t, out.String(), "CNY "+want)
							}
						}
						if tt.status == "pending" {
							assertContainsAll(t, out.String(), "pending settlement")
							assertContainsNone(t, out.String(), "CNY 0")
						} else if tt.status == "unavailable" {
							assertContainsAll(t, out.String(), "unavailable")
							assertContainsNone(t, out.String(), "CNY 0")
						}
					}
				})
			}
		}
	}
}

func TestUsageActualFeeContractErrors(t *testing.T) {
	for _, body := range []string{
		`"currency":"CNY","actualExecutionCostT":2,"finalBuyerPayableT":1`,
		`"actualExecutionCost":{"amount":"1","currency":"USD"},"finalBuyerPayable":{"amount":"2","currency":"CNY"}`,
		`"currency":"CNY","actualExecutionCostT":-1,"finalBuyerPayableT":2`,
		`"currency":"CNY","actualExecutionCostT":1,"actualExecutionCost":{"amount":"1","currency":"CNY"},"finalBuyerPayableT":20000000`,
	} {
		var raw map[string]any
		if err := json.Unmarshal([]byte(`{"transactionStatus":"succeeded",`+body+`}`), &raw); err != nil {
			t.Fatal(err)
		}
		if _, err := usageDisplayPayload(raw); err == nil {
			t.Fatalf("accepted inconsistent actual amounts: %s", body)
		}
	}
}
