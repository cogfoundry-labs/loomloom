package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreatorEarningsUsesTokenPrincipal(t *testing.T) {
	var requestedPath string
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"totalAmountT":9223372036854775807,"totalCount":0}`))
	}))
	defer server.Close()

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newCreatorEarningsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--limit", "25"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("creator earnings command error = %v", err)
	}
	if requestedPath != "/loom/v1/creators/me/earnings" {
		t.Fatalf("path=%q want creator earnings endpoint", requestedPath)
	}
	if strings.Contains(requestedQuery, "creator_user_id=") {
		t.Fatalf("query %q should not include creator_user_id", requestedQuery)
	}
	for _, want := range []string{"pageSize=25", "source=all"} {
		if !strings.Contains(requestedQuery, want) {
			t.Fatalf("query %q missing %q", requestedQuery, want)
		}
	}
	if !strings.Contains(out.String(), `"items": []`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if !strings.Contains(out.String(), `"totalAmountT": 9223372036854775807`) {
		t.Fatalf("JSON output lost int64 precision: %s", out.String())
	}
}

func TestCreatorEarningsSourceUsesAuthoritativeEndpoint(t *testing.T) {
	var requestedPath string
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"totalAmountT":0,"totalAmount":{"amount":"0","currency":"CNY"},"totalCount":0}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newCreatorEarningsCmd(opts)
	cmd.SetArgs([]string{"--source", "subscription", "--currency", "CNY", "--page-token", "next"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("creator earnings command error = %v", err)
	}
	if requestedPath != "/loom/v1/creators/me/earnings" {
		t.Fatalf("path=%q want authoritative creator earnings endpoint", requestedPath)
	}
	assertContainsAll(t, requestedQuery, "source=subscription", "currency=CNY", "pageToken=next")
}

func TestCreatorEarningsTextShowsPostedIncomeAndTotals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items":[{
				"id":"income-1",
				"source":"pay-per-use",
				"sourceEventId":"settlement_run:42",
				"amountT":4500000,
				"amount":{"amount":"0.45","currency":"CNY"},
				"currency":"CNY",
				"incomePostStatus":"posted",
				"occurredAt":"2026-09-21T10:32:15+08:00"
			}],
			"totalAmountT":4500000,
			"totalAmount":{"amount":"0.45","currency":"CNY"},
			"totalCount":1,
			"nextPageToken":"next"
		}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newCreatorEarningsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("creator earnings command error = %v", err)
	}
	assertContainsAll(t, out.String(),
		"pay-per-use", "settlement_run:42", "CNY 0.45", "posted",
		"total_amount", "total_count", "next_page_token",
	)
	assertContainsNone(t, out.String(), "4500000", `"items"`)
}

func TestCreatorEarningsRejectsUnknownSource(t *testing.T) {
	cmd := newCreatorEarningsCmd(&rootOptions{})
	cmd.SetArgs([]string{"--source", "legacy"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--source must be all") {
		t.Fatalf("error=%v want source validation", err)
	}
}

func TestCreatorEarningsRejectsOutOfRangeLimit(t *testing.T) {
	cmd := newCreatorEarningsCmd(&rootOptions{})
	cmd.SetArgs([]string{"--limit", "101"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--limit must be between 1 and 100") {
		t.Fatalf("error=%v want limit validation", err)
	}
}

func TestCreatorTransactionsTextShowsFormattedAmountsWithoutEarnings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items":[{
				"runTransactionId":"rt-1",
				"listingId":"listing-1",
				"skillName":"Writer",
				"taskFixedFeeT":5000000,
				"taskFixedFee":{"amount":"0.5000000","currency":"CNY"},
				"finalBuyerPayableT":9800000,
				"finalBuyerPayable":{"amount":"0.9800000","currency":"CNY"},
				"creatorNetEarningT":4500000,
				"currency":"CNY",
				"transactionStatus":"settled"
			}],
			"totalCount":1
		}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newCreatorTransactionsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("creator transactions command error = %v", err)
	}
	assertContainsAll(t, out.String(),
		"CNY 0.5",
		"CNY 0.98",
		"rt-1",
		"settled",
	)
	assertContainsNone(t, out.String(), "task_fixed_fee_t", "final_payable_t")
	if strings.Contains(out.String(), "4500000") {
		t.Fatalf("output=%s must not show creator net earning to buyers", out.String())
	}
}

func TestCreatorTransactionsTextUnknownCurrencyFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"runTransactionId":"rt-1","taskFixedFeeT":5000000}],"totalCount":1}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newCreatorTransactionsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("creator transactions command error = %v", err)
	}
	for _, want := range []string{"(currency unknown) 5000000", "5000000"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
}

func TestCreatorTransactionsJSONUsesMoneyWithoutRawT(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"runTransactionId":"rt-1","taskFixedFeeT":5000000,"taskFixedFee":{"amount":"0.5000000","currency":"CNY"},"creatorNetEarningT":4500000,"creatorNetEarning":{"amount":"0.4500000","currency":"CNY"}}],"totalCount":1}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newCreatorTransactionsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("creator transactions command error = %v", err)
	}
	for _, want := range []string{`"taskFixedFee": {`, `"amount": "0.5"`, `"creatorNetEarning": {`, `"amount": "0.45"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
	assertContainsNone(t, out.String(), "taskFixedFeeT", "creatorNetEarningT")
}
