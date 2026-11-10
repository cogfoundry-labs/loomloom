package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/client"
)

type marketBalanceTransport func(*http.Request) (*http.Response, error)

func (f marketBalanceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestMarketCheckBalanceOmitsRechargeLinks(t *testing.T) {
	isolateCmdConfigHome(t)
	for _, tt := range []struct {
		name, server string
	}{
		{"shengsuanyun", "https://loomloom.shengsuanyun.com/loom/v1"},
		{"cogfoundry", "https://loomloom.cogfoundry.ai/loom/v1"},
		{"custom", "https://loom.example/loom/v1"},
	} {
		for _, output := range []string{"text", "json"} {
			for _, available := range []string{"4", "6"} {
				t.Run(tt.name+"/"+output+"/"+available, func(t *testing.T) {
					original := http.DefaultTransport
					t.Cleanup(func() { http.DefaultTransport = original })
					calls := 0
					http.DefaultTransport = marketBalanceTransport(func(req *http.Request) (*http.Response, error) {
						calls++
						if req.Method != http.MethodGet || req.URL.String() != tt.server+"/users/me/balance" {
							t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
						}
						body := fmt.Sprintf(`{"currency":"CNY","availableBalance":{"amount":%q,"currency":"CNY"}}`, available)
						return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
					})
					cmd := newMarketCheckBalanceCmd(&rootOptions{server: tt.server, timeout: time.Second, output: output})
					cmd.SilenceUsage, cmd.SilenceErrors = true, true
					var out bytes.Buffer
					cmd.SetOut(&out)
					cmd.SetArgs([]string{"--call-fee", "6", "--currency", "CNY"})
					err := cmd.Execute()
					insufficient := available == "4"
					if (err != nil) != insufficient || calls != 1 {
						t.Fatalf("error=%v, calls=%d", err, calls)
					}
					if output == "json" {
						var result marketBalanceCheckResult
						if err := json.Unmarshal(out.Bytes(), &result); err != nil {
							t.Fatal(err)
						}
						if result.IsSufficient == insufficient || result.CallFee.Amount != "6" || result.AvailableBalance.Amount != available {
							t.Fatalf("unexpected balance result: %+v", result)
						}
					}
					if insufficient {
						assertContainsAll(t, err.Error(), "insufficient balance for call fee", "available CNY 4", "call fee CNY 6", "shortfall CNY 2")
					}
					assertContainsNone(t, out.String()+fmt.Sprint(err), "recharge_url", "rechargeUrl", "rechargeHint", "http://", "https://")
				})
			}
		}
	}
}

func TestMarketCheckBalance(t *testing.T) {
	for _, tt := range []struct {
		name, fee, balance, shortfall string
		sufficient                    bool
	}{
		{"enough", "6.00", "10", "0", true},
		{"exact", "6", "6", "0", true},
		{"insufficient", "6", "4", "2", false},
		{"zero fee", "0", "0", "0", true},
		{"negative balance", "0", "-1.25", "1.25", false},
		{"small difference", "0.3000001", "0.3000000", "0.0000001", false},
		{"large exact amount", "900719925.4740994", "900719925.4740993", "0.0000001", false},
		{"difference exceeds int64", "922337203685.4775807", "-922337203685.4775808", "1844674407370.9551615", false},
	} {
		for _, output := range []string{"text", "json"} {
			t.Run(tt.name+"/"+output, func(t *testing.T) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != http.MethodGet || r.URL.Path != "/loom/v1/users/me/balance" {
						t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					}
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, `{"currency":"CNY","availableBalance":{"amount":%q,"currency":"CNY"}}`, tt.balance)
				}))
				defer server.Close()
				cmd := newMarketCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: output})
				cmd.SilenceUsage, cmd.SilenceErrors = true, true
				var out, stderr bytes.Buffer
				cmd.SetOut(&out)
				cmd.SetErr(&stderr)
				cmd.SetArgs([]string{"check-balance", "--call-fee", tt.fee, "--currency", "cny"})
				err := cmd.Execute()
				if (err == nil) != tt.sufficient {
					t.Fatalf("sufficient=%t, error=%v", tt.sufficient, err)
				}
				if calls != 1 {
					t.Fatalf("got %d requests, want only one balance query", calls)
				}
				if err != nil {
					assertContainsAll(t, err.Error(), "insufficient balance for call fee", "CNY "+tt.shortfall, "recharge")
				}
				assertContainsNone(t, out.String()+stderr.String(), "estimated", "[hidden]", "availableBalanceT")
				if output == "json" {
					var result marketBalanceCheckResult
					if err := json.Unmarshal(out.Bytes(), &result); err != nil {
						t.Fatalf("invalid JSON output: %s: %v", out.String(), err)
					}
					if result.IsSufficient != tt.sufficient || result.Shortfall.Amount != tt.shortfall || result.Currency != "CNY" {
						t.Fatalf("unexpected result: %+v", result)
					}
					if result.CallFee.Amount != trimDisplayDecimal(tt.fee) || result.AvailableBalance.Amount != trimDisplayDecimal(tt.balance) {
						t.Fatalf("amounts changed: %+v", result)
					}
				} else {
					assertContainsAll(t, out.String(), "call_fee", "available_balance", "shortfall", fmt.Sprint(tt.sufficient), "CNY "+tt.shortfall)
				}
			})
		}
	}
}

func TestMarketCheckBalanceRejectsInvalidInputsBeforeRequest(t *testing.T) {
	for _, args := range [][]string{
		{}, {"--call-fee", "6"}, {"--currency", "CNY"},
		{"--call-fee", "", "--currency", "CNY"},
		{"--call-fee", "-1", "--currency", "CNY"},
		{"--call-fee", "NaN", "--currency", "CNY"},
		{"--call-fee", "1e2", "--currency", "CNY"},
		{"--call-fee", "0.00000001", "--currency", "CNY"},
		{"--call-fee", "1", "--currency", ""},
		{"--call-fee", "1", "--currency", "CN"},
		{"--call-fee", "1", "--currency", "CNY", "listing-1"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
			defer server.Close()
			cmd := newMarketCheckBalanceCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatal("invalid input accepted")
			}
			if calls != 0 {
				t.Fatalf("invalid input sent %d requests", calls)
			}
		})
	}
}

func TestMarketCheckBalanceRejectsInvalidBalance(t *testing.T) {
	for name, body := range map[string]string{
		"missing":                `{"currency":"CNY"}`,
		"null":                   `{"currency":"CNY","availableBalance":null,"availableBalanceT":null}`,
		"different currency":     `{"currency":"USD","availableBalance":{"amount":"10","currency":"USD"}}`,
		"inconsistent currency":  `{"currency":"CNY","availableBalance":{"amount":"10","currency":"USD"}}`,
		"missing currency":       `{"availableBalanceT":100000000}`,
		"money without currency": `{"currency":"CNY","availableBalance":{"amount":"10"}}`,
		"inconsistent amounts":   `{"currency":"CNY","availableBalanceT":100000000,"availableBalance":{"amount":"9","currency":"CNY"}}`,
		"invalid amount":         `{"currency":"CNY","availableBalance":{"amount":"NaN","currency":"CNY"}}`,
		"fraction":               `{"currency":"CNY","availableBalance":{"amount":"1/2","currency":"CNY"}}`,
		"excess precision":       `{"currency":"CNY","availableBalance":{"amount":"1.00000001","currency":"CNY"}}`,
		"out of range":           `{"currency":"CNY","availableBalance":{"amount":"922337203686","currency":"CNY"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, body)
			}))
			defer server.Close()
			cmd := newMarketCheckBalanceCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"--call-fee", "6", "--currency", "CNY"})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot check call fee balance") {
				t.Fatalf("expected invalid balance error, got %v", err)
			}
			if out.Len() != 0 {
				t.Fatalf("invalid balance produced a check result: %s", out.String())
			}
		})
	}
}

func TestMarketCheckBalancePropagatesAPIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":"balance unavailable"}`)
	}))
	defer server.Close()
	cmd := newMarketCheckBalanceCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--call-fee", "6", "--currency", "CNY"})
	err := cmd.Execute()
	var requestErr client.RequestError
	if !errors.As(err, &requestErr) || requestErr.StatusCode != http.StatusServiceUnavailable || out.Len() != 0 {
		t.Fatalf("API failure turned into a check result: error=%v, output=%s", err, out.String())
	}
}

func TestMarketBalanceLegacyAndPublicMoneyAgree(t *testing.T) {
	raw := flexInt64(-10_000_000)
	for _, resp := range []userBalanceSnapshotResponse{
		{Currency: "CNY", AvailableBalance: &raw},
		{Currency: "CNY", AvailableBalance: &raw, AvailableMoney: &moneyResponse{Amount: "-1.0000000", Currency: "CNY"}},
		{AvailableMoney: &moneyResponse{Amount: "-1", Currency: "CNY"}},
	} {
		amount, err := marketBalanceAmountT(resp, "CNY")
		if err != nil || amount.Int64() != -10_000_000 {
			t.Fatalf("balance=%v, error=%v", amount, err)
		}
	}
}
