package cmd

import (
	"bytes"
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
				"estimatedExecutionCostT":69300,
				"estimatedBuyerPayableT":5069300,
				"actualExecutionCostT":52000,
				"finalBuyerPayableT":5052000,
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
		"CNY 0.5000000",
		"CNY 0.5052000",
		"rt-1",
		"listing-1",
		"settled",
	)
	assertContainsNone(t, out.String(), "task_fixed_fee_t", "final_payable_t")
	if strings.Contains(out.String(), "{") {
		t.Fatalf("output=%s must not be raw JSON", out.String())
	}
}

func TestUsageListTextUnknownCurrencyFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"runTransactionId":"rt-1","taskFixedFeeT":5000000}],"totalCount":1}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newUsageListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("usage list command error = %v", err)
	}
	for _, want := range []string{"(currency unknown) 5000000", "5000000"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
}

func TestUsageListJSONPreservesRawFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"runTransactionId":"rt-1","taskFixedFeeT":5000000}],"totalCount":1,"newBackendField":"kept"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newUsageListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("usage list command error = %v", err)
	}
	for _, want := range []string{`"taskFixedFeeT": 5000000`, `"newBackendField": "kept"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
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
		"CNY 0.5000000",
		"CNY 0.5052000",
		"currency",
	)
	assertContainsNone(t, out.String(), "task_fixed_fee_t", "final_payable_t")
}
