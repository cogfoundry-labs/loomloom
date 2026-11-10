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

func TestOfficialModelCostRangeCommand(t *testing.T) {
	for _, tc := range []struct{ name, version, availability, low, high, currency string }{
		{"current", "", "available", "1.0000000", "3.0000000", "CNY"},
		{"selected", "v4", "available", "0.0000140", "0.0001076", "CNY"},
		{"single", "v4", "available", "3", "3", "CNY"},
		{"zero", "v4", "available", "0", "0", "CNY"},
		{"no data", "v4", "no_data", "", "", ""},
		{"precision", "v4", "available", "900719925.4740993", "900719925.4740994", "USD"},
	} {
		for _, output := range []string{"text", "json"} {
			t.Run(tc.name+"/"+output, func(t *testing.T) {
				response := officialModelCostRangeResponse{Version: "v4", Availability: tc.availability}
				if tc.availability == "available" {
					response.MinModelFee = &moneyResponse{Amount: tc.low, Currency: tc.currency}
					response.MaxModelFee = &moneyResponse{Amount: tc.high, Currency: tc.currency}
				}
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != "GET" || r.URL.EscapedPath() != "/loom/v1/officialTemplates/text%3F%23/modelCostRange" || r.URL.Query().Get("version") != tc.version {
						t.Errorf("unexpected request %s %s", r.Method, r.URL)
					}
					_ = json.NewEncoder(w).Encode(response)
				}))
				defer server.Close()
				command := newTemplateCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: output})
				buffer := &bytes.Buffer{}
				command.SetOut(buffer)
				command.SetErr(buffer)
				args := []string{"model-cost-range", " text?# "}
				if tc.version != "" {
					args = append(args, "--version", tc.version)
				}
				command.SetArgs(args)
				if err := command.Execute(); err != nil {
					t.Fatal(err)
				}
				if calls != 1 {
					t.Fatalf("expected one GET, got %d", calls)
				}
				if output == "json" {
					var got officialModelCostRangeResponse
					if err := json.Unmarshal(buffer.Bytes(), &got); err != nil {
						t.Fatal(err)
					}
					if got.Version != "v4" || got.Availability != tc.availability {
						t.Fatalf("unexpected response %s", buffer)
					}
					if got.MinModelFee != nil {
						low, lowErr := parseMoneyAmountT(got.MinModelFee.Amount)
						wantLow, _ := parseMoneyAmountT(tc.low)
						high, highErr := parseMoneyAmountT(got.MaxModelFee.Amount)
						wantHigh, _ := parseMoneyAmountT(tc.high)
						if lowErr != nil || highErr != nil || low != wantLow || high != wantHigh {
							t.Fatalf("amount changed: %s", buffer)
						}
					}
					if strings.Contains(buffer.String(), "listingVersionId") {
						t.Fatal("Market version leaked into official response")
					}
				} else if tc.availability == "no_data" && !strings.Contains(buffer.String(), "No historical data") {
					t.Fatalf("missing fallback %s", buffer)
				}
			})
		}
	}
}

func TestOfficialModelCostRangeCommandRejectsBadInputAndResponse(t *testing.T) {
	for _, args := range [][]string{{""}, {"../x"}, {"text", "--version", ""}, {"text", "--version", "4"}, {"text", "--version", "v0"}, {"text", "--version", "v01"}} {
		command := newTemplateModelCostRangeCmd(&rootOptions{timeout: time.Second})
		command.SilenceUsage = true
		command.SilenceErrors = true
		command.SetArgs(args)
		if err := command.Execute(); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
	for _, body := range []string{
		`{"version":"v3","availability":"no_data"}`,
		`{"version":"bad","availability":"no_data"}`,
		`{"version":"v4","availability":"available"}`,
		`{"version":"v4","availability":"available","minModelFee":{"amount":"3","currency":"CNY"},"maxModelFee":{"amount":"1","currency":"CNY"}}`,
		`{"version":"v4","availability":"available","minModelFee":{"amount":"-1","currency":"CNY"},"maxModelFee":{"amount":"3","currency":"CNY"}}`,
		`{"version":"v4","availability":"no_data","minModelFee":{"amount":"0","currency":"CNY"}}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, body) }))
		command := newTemplateModelCostRangeCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
		command.SilenceUsage = true
		command.SilenceErrors = true
		command.SetArgs([]string{"text", "--version", "v4"})
		buffer := &bytes.Buffer{}
		command.SetOut(buffer)
		if err := command.Execute(); err == nil {
			t.Errorf("accepted %s", body)
		}
		if buffer.Len() != 0 {
			t.Errorf("unexpected success output %s", buffer)
		}
		server.Close()
	}
	for _, code := range []int{401, 404, 503} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
			fmt.Fprint(w, `{"error":"lookup failed"}`)
		}))
		command := newTemplateModelCostRangeCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
		command.SilenceUsage = true
		command.SilenceErrors = true
		command.SetArgs([]string{"text"})
		buffer := &bytes.Buffer{}
		command.SetOut(buffer)
		if err := command.Execute(); err == nil {
			t.Errorf("ignored HTTP %d", code)
		}
		if buffer.Len() != 0 {
			t.Errorf("fabricated output for HTTP %d: %s", code, buffer)
		}
		server.Close()
	}
}
