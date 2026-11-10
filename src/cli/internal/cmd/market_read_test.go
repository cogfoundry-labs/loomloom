package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/client"
)

func TestMarketModelCostRangeContract(t *testing.T) {
	for _, tc := range []struct {
		name, version, availability, min, max, currency string
	}{
		{"current version", "", "available", "0.3000000", "1.8000000", "CNY"},
		{"quoted version", "quoted-version", "available", "1", "3", "CNY"},
		{"single amount", "quoted-version", "available", "3", "3", "CNY"},
		{"real zero", "quoted-version", "available", "0.0000000", "0.0000000", "CNY"},
		{"no data", "quoted-version", "no_data", "", "", ""},
		{"precision", "quoted-version", "available", "900719925.4740993", "900719925.4740994", "USD"},
	} {
		for _, output := range []string{"text", "json"} {
			t.Run(tc.name+"/"+output, func(t *testing.T) {
				selectedVersion := tc.version
				if selectedVersion == "" {
					selectedVersion = "published-version"
				}
				response := marketModelCostRangeResponse{ListingVersionID: selectedVersion, Availability: tc.availability}
				if tc.availability == "available" {
					response.MinModelFee = &moneyResponse{Amount: tc.min, Currency: tc.currency}
					response.MaxModelFee = &moneyResponse{Amount: tc.max, Currency: tc.currency}
				}
				requests := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					if r.Method != http.MethodGet || r.URL.EscapedPath() != "/loom/v1/marketListings/listing%3F%231/modelCostRange" {
						t.Errorf("unexpected request %s %s", r.Method, r.URL)
					}
					if r.URL.Query().Get("listingVersionId") != tc.version || (tc.version == "" && r.URL.RawQuery != "") {
						t.Errorf("unexpected query: %s", r.URL.RawQuery)
					}
					_ = json.NewEncoder(w).Encode(response)
				}))
				defer server.Close()
				cmd := newMarketCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: output})
				var out bytes.Buffer
				cmd.SetOut(&out)
				cmd.SetErr(new(bytes.Buffer))
				args := []string{"model-cost-range", " listing?#1 "}
				if tc.version != "" {
					args = append(args, "--listing-version-id", tc.version)
				}
				cmd.SetArgs(args)
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				if requests != 1 {
					t.Fatalf("got %d requests; expected only one history GET", requests)
				}
				if output == "json" {
					var got marketModelCostRangeResponse
					if err := json.Unmarshal(out.Bytes(), &got); err != nil {
						t.Fatal(err)
					}
					if got.ListingVersionID != selectedVersion || got.Availability != tc.availability {
						t.Fatalf("unexpected response: %s", out.String())
					}
					if tc.availability == "no_data" {
						assertContainsAll(t, out.String(), `"minModelFee": null`, `"maxModelFee": null`)
					} else if got.MinModelFee == nil || got.MaxModelFee == nil || got.MinModelFee.Amount != trimDisplayDecimal(tc.min) || got.MaxModelFee.Amount != trimDisplayDecimal(tc.max) || got.MinModelFee.Currency != tc.currency || got.MaxModelFee.Currency != tc.currency {
						t.Fatalf("historical money was lost or changed: %s", out.String())
					}
				} else {
					assertContainsAll(t, out.String(), "listing_version_id", selectedVersion, "availability", tc.availability)
					if tc.availability == "no_data" {
						assertContainsAll(t, out.String(), "No historical data available for reference")
						assertContainsNone(t, out.String(), "min_model_fee", "max_model_fee", "CNY", "USD")
					} else {
						assertContainsAll(t, out.String(), tc.currency+" "+trimDisplayDecimal(tc.min), tc.currency+" "+trimDisplayDecimal(tc.max), "per task", "actual usage may exceed")
					}
				}
			})
		}
	}
}

func TestMarketModelCostRangeRejectsInvalidResponse(t *testing.T) {
	for _, body := range []string{
		`{}`, `null`, `not json`,
		`{"listingVersionId":"other","availability":"no_data"}`,
		`{"listingVersionId":"v1","availability":"unknown"}`,
		`{"listingVersionId":"v1","availability":"available","minModelFee":null,"maxModelFee":null}`,
		`{"listingVersionId":"v1","availability":"no_data","minModelFee":{"amount":"0","currency":"CNY"}}`,
		`{"listingVersionId":"v1","availability":"available","minModelFee":{"amount":"3","currency":"CNY"},"maxModelFee":{"amount":"1","currency":"CNY"}}`,
		`{"listingVersionId":"v1","availability":"available","minModelFee":{"amount":"-1","currency":"CNY"},"maxModelFee":{"amount":"3","currency":"CNY"}}`,
		`{"listingVersionId":"v1","availability":"available","minModelFee":{"amount":"0","currency":"CNY"},"maxModelFee":{"amount":"3","currency":"USD"}}`,
		`{"listingVersionId":"v1","availability":"available","minModelFee":{"amount":"0","currency":""},"maxModelFee":{"amount":"3","currency":""}}`,
		`{"listingVersionId":"v1","availability":"available","minModelFee":{"amount":"0","currency":"CNY"},"maxModelFee":{"amount":"invalid","currency":"CNY"}}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, body) }))
			defer server.Close()
			cmd := newMarketModelCostRangeCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"listing-1", "--listing-version-id", "v1"})
			if err := cmd.Execute(); err == nil || out.Len() != 0 {
				t.Fatalf("invalid response must fail without amounts: err=%v output=%s", err, out.String())
			}
		})
	}
}

func TestMarketModelCostRangeRejectsInvalidInputBeforeHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { t.Errorf("unexpected request: %s", r.URL) }))
	defer server.Close()
	for _, args := range [][]string{{}, {""}, {"bad/id"}, {".."}, {"bad\x00id"}, {"listing", "extra"}, {"listing", "--listing-version-id", " "}, {"listing", "--listing-version-id", "v\x00id"}} {
		cmd := newMarketModelCostRangeCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("expected invalid input error: %q", args)
		}
	}
}

func TestMarketModelCostRangeFailureDoesNotFabricateData(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
				_, _ = fmt.Fprint(w, `{"error":"history unavailable"}`)
			}))
			defer server.Close()
			cmd := newMarketModelCostRangeCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"listing-1"})
			err := cmd.Execute()
			var requestErr client.RequestError
			if !errors.As(err, &requestErr) || requestErr.StatusCode != code || out.Len() != 0 {
				t.Fatalf("error=%v output=%s", err, out.String())
			}
		})
	}
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer server.Close()
		cmd := newMarketModelCostRangeCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: 50 * time.Millisecond})
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"listing-1"})
		if err := cmd.Execute(); !errors.Is(err, context.DeadlineExceeded) || out.Len() != 0 {
			t.Fatalf("error=%v output=%s", err, out.String())
		}
	})
}

// Fixture follows skill_catalog_http.go's public HTTP projection, not the
// internal gRPC message (which contains raw T amounts and creator user IDs).
const catalogPageFixture = `{"items":[
 {"source":"official","resourceId":"write-copy","displayName":"Official writer","description":"Write copy","category":"writing","categoryLabel":"写作","isOfficial":true,"pinPosition":0,"creator":{"nickname":"胜算云官方"},"successfulTaskCount":123,"effectivePublishedAt":null,"invocationFee":{"amount":"0.0000000","currency":"CNY","unit":"task"}},
 {"source":"market","resourceId":"listing-1","displayName":"Market writer","description":"Custom copy","category":"writing","categoryLabel":"写作","isOfficial":false,"pinPosition":2,"creator":{"nickname":"Creator"},"successfulTaskCount":9,"effectivePublishedAt":"2026-10-10T12:00:00Z","invocationFee":{"amount":"0.5000000","currency":"CNY","unit":"task"}}
],"page":2,"pageSize":3,"totalCount":17,"categoryOptions":[{"code":"writing","label":"写作"},{"code":"future_category","label":"新增分类"}],"futureField":"preserved"}`

func TestMarketReadListContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/loom/v1/skillCatalog" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		query := r.URL.Query()
		for key, want := range map[string]string{"keyword": "写作 %_", "category": "writing", "sortBy": "usage", "page": "2", "pageSize": "3"} {
			if got := query.Get(key); got != want {
				t.Errorf("%s=%q want %q", key, got, want)
			}
		}
		if len(query) != 5 {
			t.Errorf("unexpected query keys %v", query)
		}
		_, _ = fmt.Fprint(w, catalogPageFixture)
	}))
	defer server.Close()
	cmd := newMarketCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list", "--keyword", " 写作 %_ ", "--category", "writing", "--sort-by", "usage", "--page", "2", "--page-size", "3"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, out.String(), "official", "write-copy", "listing-1", "successful_tasks", "123", "pin_position", "CNY 0 / task", "CNY 0.5 / task", "total_count", "17", "future_category", "新增分类")
	assertContainsNone(t, out.String(), "next_page_token", "run_count", "<nil>")
}

func TestMarketReadJSONPreservesServerFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, catalogPageFixture) }))
	defer server.Close()
	cmd := newMarketListCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, out.String(), `"futureField": "preserved"`, `"effectivePublishedAt": null`, `"amount": "0.5"`, `"totalCount": 17`)
}

func TestMarketReadRejectsInvalidArgumentsBeforeHTTP(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; t.Errorf("unexpected HTTP request %s", r.URL) }))
	defer server.Close()
	for _, args := range [][]string{
		{"list", "--page", "0"}, {"list", "--page", "-1"}, {"list", "--page-size", "0"}, {"list", "--page-size", "16"},
		{"list", "--sort-by", "createdAt desc"}, {"list", "--category", "写作"}, {"list", "--category", ""},
		{"list", "--keyword", strings.Repeat("中", 201)}, {"list", "--keyword", "a\x00b"},
		{"list", "--page", "9223372036854775807"}, {"list", "--page-token", "next"}, {"list", "unexpected"},
		{"list", "--limit", "5"}, {"list", "--order-by", "createdAt desc"}, {"catalog"}, {"categories"},
		{"show", "template-1", "--source", "private"}, {"show", ""}, {"show", "bad/id"}, {"show", "bad\x00id", "--source", "official"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := newMarketCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("expected error for %q", args)
			}
		})
	}
	if calls != 0 {
		t.Fatalf("HTTP calls=%d", calls)
	}
}

func TestMarketReadShowPublicDetailAndPricing(t *testing.T) {
	for _, source := range []string{"official", "market"} {
		t.Run(source, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/loom/v1/skillCatalog/"+source+"/resource-1" || r.URL.RawQuery != "" {
					t.Errorf("unexpected URL %s", r.URL)
				}
				fixture := fmt.Sprintf(`{"source":%q,"resourceId":"resource-1","isOfficial":%t,"displayName":"Writer","category":"writing","categoryLabel":"写作","successfulTaskCount":42,"effectivePublishedAt":null,
 "value":{"targetAudience":"Authors","problemSolved":"Drafting","scenarios":["Articles"],"notSuitableFor":"Legal advice"},
 "input":{"summary":"Upload text","fields":[{"key":"prompt","name":"Prompt","required":true,"type":"text","multiValue":false,"acceptedMimeTypes":["text/plain"],"inputHint":"UTF-8"}]},"columns":[{"fieldKey":"prompt","headerLabel":"Prompt","order":1}],"instructions":["Enter text"],"sampleRows":[{"values":{"prompt":"Hi"}}],"outputType":"text","outputs":[{"key":"draft","description":"Draft text"}],"trust":{"runCount":null,"downloadCount":null,"updates":[]},"skillPackage":{"available":false},
 "pricing":{"creatorFee":{"amount":"0.5000000","currency":"CNY","unit":"task"},"payPerUseEnabled":null,"subscriptionOffersStatus":"available","subscriptionOffers":[{"bundleId":"bundle-1","bundleName":"Writer pack","revision":7,"tiers":[{"tierType":"monthly","price":{"amount":"12.5000","currency":"CNY"},"enabled":true,"durationDays":30}]}],"subscriptionPolicy":{"paymentTiming":"onPurchase","creatorFeeWaived":true,"modelFeeIncluded":false},"modelFee":{"billingMode":"actualUsage","settledBy":"platform"}}}`, source, source == "official")
				var body map[string]any
				if err := json.Unmarshal([]byte(fixture), &body); err != nil {
					t.Fatal(err)
				}
				if source == "official" {
					body["templateId"] = "resource-1"
					pricing := body["pricing"].(map[string]any)
					pricing["creatorFee"].(map[string]any)["amount"] = "0.0000000"
					pricing["payPerUseEnabled"] = true
					pricing["subscriptionOffersStatus"] = "notApplicable"
					pricing["subscriptionOffers"] = []any{}
				}
				_ = json.NewEncoder(w).Encode(body)
			}))
			defer server.Close()
			cmd := newMarketShowCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"resource-1", "--source", source})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			assertContainsAll(t, out.String(), "resource-1", "Authors", "Drafting", "Legal advice", "UTF-8", "acceptedMimeTypes", "Draft text", "actualUsage")
			if source == "official" {
				assertContainsAll(t, out.String(), "CNY 0 / task", "notApplicable", "template_id")
				assertContainsNone(t, out.String(), "bundle-1", "monthly")
			} else {
				assertContainsAll(t, out.String(), "CNY 0.5 / task", "CNY 12.5", "monthly", "bundle-1", "revision=7")
			}
			assertContainsNone(t, out.String(), "input_schema_error", "input_schema_version")
			for _, line := range []string{"pay_per_use_enabled", "subscription_model_fee_included"} {
				if !strings.Contains(out.String(), line) {
					t.Fatalf("missing %s", line)
				}
			}
		})
	}
}

func TestMarketReadUnavailableOffersAreNotInvented(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"source":"market","resourceId":"listing-1","pricing":{"creatorFee":{"amount":"0","currency":"USD","unit":"task"},"payPerUseEnabled":null,"subscriptionOffersStatus":"unavailable","subscriptionOffers":[]}}`)
	}))
	defer server.Close()
	cmd := newMarketShowCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, out.String(), "unavailable", "unknown", "USD 0 / task")
	assertContainsNone(t, out.String(), "monthly", "CNY", "subscription_offer\t")
}

func TestMarketReadPropagatesFailureWithoutFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, `{"error":"catalog statistics unavailable"}`)
	}))
	defer server.Close()
	cmd := newMarketListCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	cmd.SilenceUsage = true
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "catalog statistics unavailable") {
		t.Fatalf("error=%v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("failure produced success output: %s", out.String())
	}
}

func TestCatalogFeeRejectsMissingCurrencyAmountOrUnsupportedUnit(t *testing.T) {
	for _, fee := range []*skillCatalogFee{nil, {Amount: "0.5", Unit: "task"}, {Amount: "invalid", Currency: "CNY", Unit: "task"}, {Amount: "0", Currency: "CNY", Unit: "run"}} {
		if _, err := formatCatalogFee(fee); err == nil {
			t.Fatalf("invalid fee accepted: %+v", fee)
		}
	}
}

func TestMarketListDefaultsAndCategoriesOnEmptyPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/loom/v1/skillCatalog" || r.URL.RawQuery != "page=1&pageSize=15&sortBy=default" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		_, _ = fmt.Fprint(w, `{"items":[],"page":1,"pageSize":15,"totalCount":0,"categoryOptions":[{"code":"new_code","label":"新增分类"}]}`)
	}))
	defer server.Close()
	cmd := newMarketListCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, out.String(), "no catalog skills", "new_code", "新增分类", "total_count", "page_size")
}

func TestMarketListPreservesInt64Counts(t *testing.T) {
	for _, output := range []string{"text", "json"} {
		t.Run(output, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprint(w, strings.ReplaceAll(catalogPageFixture, `"successfulTaskCount":123`, `"successfulTaskCount":9007199254740993`))
			}))
			defer server.Close()
			cmd := newMarketListCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: output})
			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			assertContainsAll(t, out.String(), "9007199254740993")
			assertContainsNone(t, out.String(), "9007199254740992")
		})
	}
}

func TestMarketShowDefaultSourceAndEscapedIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/loom/v1/skillCatalog/market/listing%3F%231" || r.URL.RawQuery != "" {
			t.Errorf("unexpected URL %s", r.URL)
		}
		_, _ = fmt.Fprint(w, `{"source":"market","resourceId":"listing?#1","futureField":"kept"}`)
	}))
	defer server.Close()
	cmd := newMarketShowCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{" listing?#1 "})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, out.String(), `"resourceId": "listing?#1"`, `"futureField": "kept"`)
}

func TestMarketReadRejectsInvalidResponseContract(t *testing.T) {
	for _, tt := range []struct{ command, body string }{
		{"list", `{"items":[],"nextPageToken":"legacy"}`},
		{"list", `{"items":null,"categoryOptions":[],"page":1,"pageSize":15,"totalCount":0}`},
		{"list", `{"items":[],"categoryOptions":null,"page":1,"pageSize":15,"totalCount":0}`},
		{"show", `{"id":"listing-1","inputSchemaSnapshot":"{}"}`},
		{"show", `{"source":"official","resourceId":"listing-1"}`},
		{"show", `{"source":"market","resourceId":"wrong-id"}`},
	} {
		t.Run(tt.command+tt.body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, tt.body) }))
			defer server.Close()
			cmd := newMarketCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
			cmd.SilenceUsage = true
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(new(bytes.Buffer))
			args := []string{tt.command}
			if tt.command == "show" {
				args = append(args, "listing-1")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatal("invalid response accepted")
			}
			if out.Len() != 0 {
				t.Fatalf("invalid response produced success output: %s", out.String())
			}
		})
	}
}
