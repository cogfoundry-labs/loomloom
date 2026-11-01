package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/client"
	"github.com/spf13/cobra"
)

const hiddenEstimateFields = `"estimatedCostT":987654321,"estimatedCost":{"amount":"98.7654321","currency":"CNY"},"estimatedTotalCostT":987654321,"estimatedTotalCost":{"amount":"98.7654321","currency":"CNY"},"estimatedExecutionCostT":987654321,"estimatedExecutionCost":{"amount":"98.7654321","currency":"CNY"},"estimatedBuyerPayableT":987654321,"estimatedBuyerPayable":{"amount":"98.7654321","currency":"CNY"}`

func TestExecutionCommandsHideEstimates(t *testing.T) {
	isolateCmdConfigHome(t)
	input := writeMarketInputFile(t, `{"inputRows":[{"prompt":"hello"}]}`)
	workbook := filepath.Join(t.TempDir(), "input.xlsx")
	if err := os.WriteFile(workbook, []byte("xlsx bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	rows := filepath.Join(t.TempDir(), "rows.json")
	if err := os.WriteFile(rows, []byte(`[{"prompt":"hello"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	record := `{` + hiddenEstimateFields + `,"runId":"run-1","runTransactionId":"rt-1","listingId":"listing-1","listingVersionId":"lv-1","status":"completed","currency":"CNY","actualCost":{"amount":"0.25","currency":"CNY"},"actualExecutionCost":{"amount":"0.25","currency":"CNY"},"taskFixedFee":{"amount":"0.5","currency":"CNY"},"effectiveCreatorFee":{"amount":"0.5","currency":"CNY"},"effectivePaymentMode":"pay_per_use","taskCount":1,"valid":true,"balanceCheck":{"currency":"CNY","availableBalance":10000000,"isSufficient":true}}`
	tests := []struct {
		name   string
		newCmd func(*rootOptions) *cobra.Command
		args   []string
		shape  string
	}{
		{"official rows", newRunPrecheckCmd, []string{"text-v1", "--file", rows}, "record"},
		{"official workbook", newTemplatePrecheckFileCmd, []string{"text-v1", workbook}, "record"},
		{"private rows", newTemplateSpecPrecheckCmd, []string{"tmpl", "--version-id", "ver", "--input-file-id", "input-file-1"}, "record"},
		{"private workbook", newTemplateSpecPrecheckWorkbookCmd, []string{"tmpl", "ver", workbook}, "record"},
		{"official submit", newRunExecuteCmd, []string{"text-v1", "--file", rows}, "record"},
		{"official workbook submit", newTemplateSubmitFileCmd, []string{"text-v1", workbook}, "record"},
		{"private submit", newTemplateSpecRunCmd, []string{"tmpl", "--version-id", "ver", "--input-file-id", "input-file-1"}, "record"},
		{"private workbook submit", newTemplateSpecSubmitWorkbookCmd, []string{"tmpl", "ver", workbook}, "record"},
		{"quote", newMarketQuoteCmd, []string{"listing-1", "--input-file", input}, "record"},
		{"workbook quote", newMarketWorkbookQuoteCmd, []string{"listing-1", "--file", workbook}, "record"},
		{"market preview", newMarketRunCmd, []string{"listing-1", "--input-file", input}, "record"},
		{"workbook preview", newMarketWorkbookRunCmd, []string{"listing-1", "--file", workbook}, "record"},
		{"market submit", newMarketRunCmd, []string{"listing-1", "--input-file", input, "--confirm"}, "record"},
		{"workbook submit", newMarketWorkbookRunCmd, []string{"listing-1", "--file", workbook, "--confirm"}, "record"},
		{"market retry", newMarketRunCmd, []string{"listing-1", "--input-file", input, "--confirm", "--client-request-id", "retry-1"}, "lookup"},
		{"workbook retry", newMarketWorkbookRunCmd, []string{"listing-1", "--file", workbook, "--confirm", "--client-request-id", "retry-1"}, "lookup"},
		{"usage list", newUsageListCmd, nil, "list"},
		{"usage get", newUsageGetCmd, []string{"rt-1"}, "record"},
		{"run list", newRunListCmd, nil, "list"},
		{"run get", newRunGetCmd, []string{"run-1"}, "run"},
		{"run watch", newRunWatchCmd, []string{"run-1"}, "run"},
		{"creator transactions", newCreatorTransactionsCmd, nil, "list"},
	}
	for _, tt := range tests {
		for _, output := range []string{"text", "json"} {
			t.Run(tt.name+"/"+output, func(t *testing.T) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					w.Header().Set("Content-Type", "application/json")
					if r.URL.Path == "/loom/v1/marketListings/listing-1" {
						_, _ = w.Write([]byte(marketListingDetailBody(t)))
						return
					}
					if strings.HasSuffix(r.URL.Path, "/schema") {
						_, _ = w.Write([]byte(`{"templateId":"text-v1"}`))
						return
					}
					if strings.HasSuffix(r.URL.Path, "/marketTransactions:lookup") {
						if tt.shape == "lookup" {
							_, _ = w.Write([]byte(`{"recoveryAction":"none","transaction":` + record + `}`))
						} else {
							http.NotFound(w, r)
						}
						return
					}
					if tt.shape == "lookup" {
						t.Errorf("retry unexpectedly called %s", r.URL.Path)
					}
					switch tt.shape {
					case "run":
						_, _ = w.Write([]byte(`{"run":` + record + `,"market":` + record + `}`))
					case "list":
						_, _ = w.Write([]byte(`{"items":[` + record + `],"totalCount":1}`))
					default:
						_, _ = w.Write([]byte(record))
					}
				}))
				defer server.Close()
				cmd := tt.newCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: output})
				var out, stderr bytes.Buffer
				cmd.SetOut(&out)
				cmd.SetErr(&stderr)
				cmd.SetArgs(tt.args)
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				assertContainsNone(t, out.String()+stderr.String(), "estimated", "987654321", "98.7654321")
				if calls == 0 || out.Len() == 0 {
					t.Fatalf("missing request/output: calls=%d output=%s", calls, out.String())
				}
				if output == "json" {
					var payload any
					if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestExecutionJSONDoesNotMutateQuoteOrUserData(t *testing.T) {
	quote := map[string]any{"estimatedCostT": 987654321, "effectiveCreatorFee": map[string]any{"amount": "0.5", "currency": "CNY"}, "listingVersionId": "lv-1", "effectivePaymentMode": "pay_per_use"}
	response := map[string]any{"quote": quote, "input": map[string]any{"estimatedCost": "user supplied"}, "result": map[string]any{"estimatedCost": "generated content"}}
	var out bytes.Buffer
	if err := writeExecutionJSON(&out, response); err != nil {
		t.Fatal(err)
	}
	assertContainsNone(t, out.String(), "987654321")
	assertContainsAll(t, out.String(), "user supplied", "generated content", "lv-1", "pay_per_use", "0.5")
	if quote["estimatedCostT"] != 987654321 {
		t.Fatal("display filtering mutated quote")
	}
	payload := marketExecutePayload(marketInputPayload{}, "request-1", "pay_per_use", quote)
	if payload["expectedListingVersionId"] != "lv-1" || payload["expectedPaymentMode"] != "pay_per_use" || payload["maxCreatorFee"] == nil {
		t.Fatalf("execution guard lost: %#v", payload)
	}
}

func TestTextOutputFiltersCopiesOfCompleteResponses(t *testing.T) {
	const body = `{` + hiddenEstimateFields + `,"runId":"run-1","transactionStatus":"succeeded","finalBuyerPayable":{"amount":"0.75","currency":"CNY"},"currency":"CNY","taskCount":9007199254740993,"taskFixedFee":{"amount":"0.5","currency":"CNY"},"actualCost":{"amount":"0.25","currency":"CNY"},"actualExecutionCost":{"amount":"0.25","currency":"CNY"},"balanceCheck":{"currency":"CNY","availableBalance":10000000,"isSufficient":false},"errorMessage":"insufficient balance: required 98.7654321 CNY"}`
	var precheck precheckTemplateRowsResponse
	var run runStatusResponse
	var usage marketUsageSummary
	var creator creatorTransactionSummary
	var quote map[string]any
	for _, response := range []any{&precheck, &run, &usage, &creator, &quote} {
		decoder := json.NewDecoder(strings.NewReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(response); err != nil {
			t.Fatal(err)
		}
	}
	// Receiving types must retain both public Money and legacy raw amounts.
	for name, fields := range map[string]struct {
		money *moneyResponse
		raw   *flexInt64
	}{
		"precheck":          {precheck.EstimatedTotalCost, precheck.EstimatedTotalCostT},
		"run":               {run.EstimatedCost, run.EstimatedCostT},
		"usage execution":   {usage.EstimatedExecutionCost, usage.EstimatedExecutionCostT},
		"usage payable":     {usage.EstimatedBuyerPayable, usage.EstimatedBuyerPayableT},
		"creator execution": {creator.EstimatedExecutionCost, creator.EstimatedExecutionCostT},
		"creator payable":   {creator.EstimatedBuyerPayable, creator.EstimatedBuyerPayableT},
	} {
		if fields.money == nil || fields.money.Amount != "98.7654321" || fields.raw == nil || *fields.raw != 987654321 {
			t.Fatalf("%s lost received estimate: %#v", name, fields)
		}
	}
	for _, tt := range []struct {
		name     string
		response any
		print    func(io.Writer) error
		kept     []string
	}{
		{"precheck", &precheck, func(w io.Writer) error { return printPrecheck(w, precheck) }, []string{"available_balance", "CNY 1", "false"}},
		{"run", &run, func(w io.Writer) error { return printRunSummary(w, run) }, []string{"insufficient balance", "CNY 0.25"}},
		{"usage", &usage, func(w io.Writer) error { return printUsageDetail(w, usage) }, []string{"CNY 0.5", "CNY 0.25"}},
		{"quote", quote, func(w io.Writer) error { return printMarketQuote(w, quote) }, []string{"CNY 0.5", "9007199254740993"}},
		{"execution", quote, func(w io.Writer) error { return printMarketExecution(w, quote) }, []string{"CNY 0.5", "CNY 0.25"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := tt.print(&out); err != nil {
				t.Fatal(err)
			}
			assertContainsNone(t, out.String(), "estimated", "98.7654321", "987654321")
			assertContainsAll(t, out.String(), tt.kept...)
			after, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("printing changed original response: %s -> %s", before, after)
			}
		})
	}
}

func TestSubmissionBalanceErrorRemainsFailureWithoutEstimate(t *testing.T) {
	isolateCmdConfigHome(t)
	const body = `{"code":"insufficient_balance","error":"insufficient balance: current 1.0000 CNY, required 98.7654 CNY"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/schema") {
			_, _ = w.Write([]byte(`{"templateId":"text-v1"}`))
			return
		}
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	rows := writeMarketInputFile(t, `[{"prompt":"hello"}]`)
	cmd := newRunExecuteCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"text-v1", "--file", rows})
	err := cmd.Execute()
	var requestErr client.RequestError
	if !errors.As(err, &requestErr) || requestErr.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("failure changed: %v", err)
	}
	if requestErr.Body != body {
		t.Fatal("raw error changed")
	}
	assertContainsAll(t, err.Error(), "insufficient balance", "insufficient_balance", "402")
	assertContainsNone(t, err.Error()+out.String()+stderr.String(), "98.7654", "required", "run_id")
}
