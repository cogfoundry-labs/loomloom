package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func writeInitialTiersFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tiers.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInitialSubscriptionTiersValidateMoneyAndExplicitFields(t *testing.T) {
	for _, tt := range []struct {
		name, content, wantError string
	}{
		{"zero", `[{"tierType":"monthly","price":{"amount":"0","currency":"CNY"},"enabled":true}]`, ""},
		{"minimum", `[{"tierType":"yearly","price":{"amount":"0.0001","currency":"CNY"},"enabled":true}]`, ""},
		{"four_decimals", `[{"tierType":"monthly","price":{"amount":"0.0100","currency":"CNY"},"enabled":true}]`, ""},
		{"both_enabled", `[{"tierType":"monthly","price":{"amount":"0.01","currency":"CNY"},"enabled":true},{"tierType":"yearly","price":{"amount":"0.02","currency":"CNY"},"enabled":true}]`, ""},
		{"mixed_enabled", `[{"tierType":"monthly","price":{"amount":"0.01","currency":"CNY"},"enabled":true},{"tierType":"yearly","price":{"amount":"0.02","currency":"CNY"},"enabled":false}]`, ""},
		{"all_disabled_forwarded", `[{"tierType":"monthly","price":{"amount":"0","currency":"CNY"},"enabled":false}]`, ""},
		{"null_array", `null`, "null and empty arrays"},
		{"empty_array", `[]`, "null and empty arrays"},
		{"object", `{"subscriptionTiers":[]}`, "JSON array"},
		{"trailing_document", `[] []`, "exactly one JSON array"},
		{"malformed", `[`, "JSON array"},
		{"unknown_field", `[{"tierType":"monthly","durationDays":30}]`, "unknown field"},
		{"invalid_type", `[{"tierType":"weekly","price":{"amount":"0.01","currency":"CNY"},"enabled":true}]`, "monthly or yearly"},
		{"duplicate_monthly", `[{"tierType":"monthly","price":{"amount":"0","currency":"CNY"},"enabled":true},{"tierType":"monthly","price":{"amount":"1","currency":"CNY"},"enabled":false}]`, "duplicate monthly"},
		{"duplicate_yearly", `[{"tierType":"yearly","price":{"amount":"0","currency":"CNY"},"enabled":true},{"tierType":"yearly","price":{"amount":"1","currency":"CNY"},"enabled":true}]`, "duplicate yearly"},
		{"missing_price", `[{"tierType":"monthly","enabled":true}]`, "price is required"},
		{"null_price", `[{"tierType":"monthly","price":null,"enabled":true}]`, "price is required"},
		{"missing_amount", `[{"tierType":"monthly","price":{"currency":"CNY"},"enabled":true}]`, "decimal string"},
		{"number_amount", `[{"tierType":"monthly","price":{"amount":0.01,"currency":"CNY"},"enabled":true}]`, "JSON array"},
		{"negative", `[{"tierType":"monthly","price":{"amount":"-0.01","currency":"CNY"},"enabled":true}]`, "non-negative"},
		{"precision", `[{"tierType":"monthly","price":{"amount":"0.00001","currency":"CNY"},"enabled":true}]`, "four decimal places"},
		{"overflow", `[{"tierType":"monthly","price":{"amount":"922337203685.4776","currency":"CNY"},"enabled":true}]`, "too large"},
		{"exponent", `[{"tierType":"monthly","price":{"amount":"1e-2","currency":"CNY"},"enabled":true}]`, "decimal string"},
		{"missing_currency", `[{"tierType":"monthly","price":{"amount":"0"},"enabled":true}]`, "currency must be CNY"},
		{"wrong_currency", `[{"tierType":"monthly","price":{"amount":"0.01","currency":"USD"},"enabled":true}]`, "currency must be CNY"},
		{"missing_enabled", `[{"tierType":"monthly","price":{"amount":"0.01","currency":"CNY"}}]`, "explicit boolean"},
		{"null_enabled", `[{"tierType":"monthly","price":{"amount":"0.01","currency":"CNY"},"enabled":null}]`, "explicit boolean"},
		{"string_enabled", `[{"tierType":"monthly","price":{"amount":"0.01","currency":"CNY"},"enabled":"true"}]`, "JSON array"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tiers, err := readInitialSubscriptionTiersFile(writeInitialTiersFile(t, tt.content))
			if tt.wantError == "" {
				if err != nil || len(tiers) == 0 {
					t.Fatalf("tiers=%#v err=%v", tiers, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error=%v want %q", err, tt.wantError)
			}
		})
	}
}

func TestInitialSubscriptionTiersTravelAtomicallyWithPaymentModes(t *testing.T) {
	for _, payPerUse := range []bool{false, true} {
		for _, operation := range []string{"publish", "payment-settings"} {
			t.Run(operation+"/pay-per-use="+strconv.FormatBool(payPerUse), func(t *testing.T) {
				var payload map[string]any
				mutations := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.Method == http.MethodGet && r.URL.Path == "/loom/v1/creators/me/marketListings:checkDuplicateName" {
						_, _ = w.Write([]byte(`{"matches":[]}`))
						return
					}
					wantMethod, wantPath := http.MethodPost, "/loom/v1/marketListings"
					if operation == "payment-settings" {
						wantMethod, wantPath = http.MethodPut, "/loom/v1/creators/me/marketListings/listing-1/paymentSettings"
					}
					if r.Method != wantMethod || r.URL.Path != wantPath {
						t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
						w.WriteHeader(http.StatusNotFound)
						return
					}
					mutations++
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					_, _ = w.Write([]byte(`{"id":"listing-1","reviewRequestId":"review-1"}`))
				}))
				defer server.Close()

				file := writeInitialTiersFile(t, `[{"tierType":"monthly","price":{"amount":"0","currency":"CNY"},"enabled":true},{"tierType":"yearly","price":{"amount":"0.0001","currency":"CNY"},"enabled":false}]`)
				cmd, args := initialTiersCommand(operation, server.URL)
				args = append(args, "--pay-per-use="+strconv.FormatBool(payPerUse), "--subscription=true", "--subscription-tiers-file", file)
				cmd.SetArgs(args)
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				if mutations != 1 || payload["payPerUse"] != payPerUse || payload["subscription"] != true {
					t.Fatalf("mutations=%d payload=%#v", mutations, payload)
				}
				tiers := payload["subscriptionTiers"].([]any)
				monthly, yearly := tiers[0].(map[string]any), tiers[1].(map[string]any)
				if monthly["tierType"] != "monthly" || monthly["enabled"] != true || yearly["tierType"] != "yearly" || yearly["enabled"] != false {
					t.Fatalf("tiers=%#v", tiers)
				}
				if price := monthly["price"].(map[string]any); price["amount"] != "0" || price["currency"] != "CNY" {
					t.Fatalf("zero price lost or coerced: %#v", price)
				}
				if price := yearly["price"].(map[string]any); price["amount"] != "0.0001" || price["currency"] != "CNY" {
					t.Fatalf("price precision lost: %#v", price)
				}
				if operation == "payment-settings" && payload["expectedRevision"] != float64(0) {
					t.Fatalf("initial revision lost: %#v", payload)
				}
			})
		}
	}
}

func TestInitialSubscriptionTiersLocalRejectionsMakeNoRequests(t *testing.T) {
	file := writeInitialTiersFile(t, `[{"tierType":"monthly","price":{"amount":"0.01","currency":"CNY"},"enabled":true}]`)
	invalid := writeInitialTiersFile(t, `[{"tierType":"monthly","price":{"amount":"0.00001","currency":"CNY"},"enabled":true}]`)
	for _, tt := range []struct {
		name, operation, wantError string
		flags                      []string
	}{
		{"existing_listing", "publish", "only for first publication", []string{"--listing-id", "listing-1", "--subscription=true", "--subscription-tiers-file", file}},
		{"subscription_off_publish", "publish", "requires --subscription=true", []string{"--subscription=false", "--subscription-tiers-file", file}},
		{"subscription_off_settings", "payment-settings", "requires --subscription=true", []string{"--pay-per-use=true", "--subscription=false", "--subscription-tiers-file", file}},
		{"invalid_money_publish", "publish", "four decimal places", []string{"--subscription=true", "--subscription-tiers-file", invalid}},
		{"invalid_money_settings", "payment-settings", "four decimal places", []string{"--pay-per-use=false", "--subscription=true", "--subscription-tiers-file", invalid}},
		{"empty_path", "publish", "file path", []string{"--subscription=true", "--subscription-tiers-file="}},
		{"missing_file", "publish", "read --subscription-tiers-file", []string{"--subscription=true", "--subscription-tiers-file", filepath.Join(t.TempDir(), "missing.json")}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
			defer server.Close()
			cmd, args := initialTiersCommand(tt.operation, server.URL)
			cmd.SetArgs(append(args, tt.flags...))
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.wantError) || requests != 0 {
				t.Fatalf("error=%v requests=%d", err, requests)
			}
		})
	}
}

func TestInitialSubscriptionTiersPreserveServerPriceGate(t *testing.T) {
	for _, operation := range []string{"publish", "payment-settings"} {
		for _, withDisabledTiers := range []bool{false, true} {
			t.Run(operation+"/disabled-file="+strconv.FormatBool(withDisabledTiers), func(t *testing.T) {
				mutations := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.Method == http.MethodGet {
						_, _ = w.Write([]byte(`{"matches":[]}`))
						return
					}
					mutations++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					_, hasTiers := body["subscriptionTiers"]
					if hasTiers != withDisabledTiers {
						t.Errorf("unexpected subscriptionTiers presence: %#v", body)
					}
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte(`{"code":"subscription_tier_required","error":"请至少配置并启用一个订阅价格档位（月度或年度）后，再开启订阅模式、提交审核或上架。"}`))
				}))
				defer server.Close()
				cmd, args := initialTiersCommand(operation, server.URL)
				args = append(args, "--pay-per-use=false", "--subscription=true")
				if withDisabledTiers {
					file := writeInitialTiersFile(t, `[{"tierType":"monthly","price":{"amount":"0","currency":"CNY"},"enabled":false}]`)
					args = append(args, "--subscription-tiers-file", file)
				}
				cmd.SetArgs(args)
				err := cmd.Execute()
				if err == nil || !strings.Contains(err.Error(), "status=409") || !strings.Contains(err.Error(), "subscription_tier_required") || mutations != 1 {
					t.Fatalf("error=%v mutations=%d", err, mutations)
				}
			})
		}
	}
}

func TestExistingPublicationOmitsUnchangedPaymentModesAndInitialTiers(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/loom/v1/creators/me/marketListings/listing-1" {
			_, _ = w.Write([]byte(`{"publishedVersionId":""}`))
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/loom/v1/marketListings" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(`{"id":"listing-1"}`))
	}))
	defer server.Close()
	cmd, args := initialTiersCommand("publish", server.URL)
	cmd.SetArgs(append(args, "--listing-id", "listing-1"))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"payPerUse", "subscription", "subscriptionTiers", "expectedPaymentSettingsRevision"} {
		if _, ok := payload[key]; ok {
			t.Errorf("unchanged %s should be omitted: %#v", key, payload)
		}
	}
}

func TestInitialSubscriptionTiersDoNotRetryRevisionOrExistingPriceRejections(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		status     int
	}{
		{"stale_revision", `{"code":"revision_conflict","error":"revision conflict"}`, http.StatusConflict},
		{"existing_prices", `{"error":"已有订阅价格档位请通过订阅包档位设置修改，支付模式设置不能覆盖已有价格。"}`, http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodPut || r.URL.Path != "/loom/v1/creators/me/marketListings/listing-1/paymentSettings" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["expectedRevision"] != float64(2) {
					t.Errorf("revision was changed: %#v", body)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			cmd, _ := initialTiersCommand("payment-settings", server.URL)
			file := writeInitialTiersFile(t, `[{"tierType":"monthly","price":{"amount":"0.01","currency":"CNY"},"enabled":true}]`)
			cmd.SetArgs([]string{"listing-1", "--pay-per-use=false", "--subscription=true", "--expected-revision", "2", "--subscription-tiers-file", file})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), tt.body) || requests != 1 {
				t.Fatalf("error=%v requests=%d", err, requests)
			}
		})
	}
}

func initialTiersCommand(operation, serverURL string) (*cobra.Command, []string) {
	opts := &rootOptions{server: serverURL + "/loom/v1", timeout: time.Second, output: "json"}
	var cmd *cobra.Command
	var args []string
	if operation == "publish" {
		cmd = newListingPublishCmd(opts)
		args = []string{"template-1", "--template-version-id", "version-1", "--display-name", "Subscription Test", "--task-fixed-fee", "0.01"}
	} else {
		cmd = newListingPaymentSettingsCmd(opts)
		args = []string{"listing-1", "--expected-revision", "0"}
	}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	return cmd, args
}
