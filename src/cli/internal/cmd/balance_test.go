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
		_, _ = w.Write([]byte(`{"currency":"CNY","settledBalanceT":120000000,"settledBalance":{"amount":"12.0000000","currency":"CNY"},"availability":"settled_only","finalAdmission":"gateway"}`))
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
	for _, want := range []string{"settled_balance", "CNY 12.0000000", "settled_only", "gateway"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("output=%q missing %q", out.String(), want)
		}
	}
	if bytes.Contains(out.Bytes(), []byte("available_balance")) {
		t.Fatalf("output must not claim available balance: %q", out.String())
	}
}
