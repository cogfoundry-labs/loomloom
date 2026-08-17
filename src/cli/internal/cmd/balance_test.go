package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBalanceCmdUsesSettledSnapshotEndpoint(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"currency":"CNY","settledBalanceT":120000000,"settledBalance":{"amount":"12.0000000","currency":"CNY"},"pendingModelChargesT":20000000,"pendingModelCharges":{"amount":"2.0000000","currency":"CNY"},"availableBalanceT":100000000,"availableBalance":{"amount":"10.0000000","currency":"CNY"},"availability":"model_pending_only","incompletePendingCategories":["creator_surcharge"],"finalAdmission":"gateway"}`))
	}))
	defer server.Close()

	cmd := newBalanceCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("balance command error = %v", err)
	}
	if requestedPath != "/loom/v1/users/me/balance" {
		t.Fatalf("path=%q want /loom/v1/users/me/balance", requestedPath)
	}
	for _, want := range []string{"settled_balance", "CNY 12.0000000", "pending_model_charges", "CNY 2.0000000", "available_after_model_pending", "CNY 10.0000000", "creator_surcharge", "gateway"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("output=%q missing %q", out.String(), want)
		}
	}
	if bytes.Contains(out.Bytes(), []byte("available_balance")) {
		t.Fatalf("output must not claim complete available balance: %q", out.String())
	}
}

func TestFormatSignedBalanceMoneyAllowsNegativeValue(t *testing.T) {
	amount := flexInt64(-12_345_678)
	if got := formatSignedBalanceMoney(&amount, "CNY"); got != "CNY -1.2345678" {
		t.Fatalf("formatSignedBalanceMoney() = %q", got)
	}
}
