package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestListingPublishCategoryAndPublicValueContract(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loom/v1/creators/me/marketListings:checkDuplicateName":
			_, _ = fmt.Fprint(w, `{"matches":[]}`)
		case "/loom/v1/marketListings":
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			_, _ = fmt.Fprint(w, `{"id":"listing-1","reviewRequestId":"review-1","reviewStatus":"pending"}`)
		default:
			t.Errorf("unexpected URL %s", r.URL)
		}
	}))
	defer server.Close()
	cmd := newListingPublishCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"template-1", "--template-version-id", "version-1", "--display-name", "Writer", "--task-fixed-fee", "0", "--category", " new_category ", "--target-audience", " Authors ", "--problem-solved", " Drafts ", "--scenario", "News, articles", "--scenario", "Emails", "--not-suitable-for", "Legal advice", "--update-summary", " Initial release "})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if body["category"] != "new_category" || body["updateSummary"] != "Initial release" {
		t.Fatalf("body=%v", body)
	}
	value := body["value"].(map[string]any)
	if value["targetAudience"] != "Authors" || value["problemSolved"] != "Drafts" || value["notSuitableFor"] != "Legal advice" || !reflect.DeepEqual(value["scenarios"], []any{"News, articles", "Emails"}) {
		t.Fatalf("value=%v", value)
	}
	assertContainsAll(t, out.String(), "review-1", "pending")
	for _, key := range []string{"tags", "categoryLabel", "pinPosition", "category_code"} {
		if _, ok := body[key]; ok {
			t.Fatalf("invalid public request field %s", key)
		}
	}
}

func TestListingPublishOmittedCategoryDoesNotChangeLegacyPayload(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, `{"matches":[]}`)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer server.Close()
	cmd := newListingPublishCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"template-1", "--template-version-id", "v1", "--display-name", "Bot", "--task-fixed-fee", "0"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"category", "value", "updateSummary"} {
		if _, ok := body[key]; ok {
			t.Fatalf("omitted flag sent %s: %v", key, body)
		}
	}
}

func TestListingRepublishPreservesValueAndOmittedCategory(t *testing.T) {
	for _, explicitPackage := range []bool{false, true} {
		t.Run(fmt.Sprint(explicitPackage), func(t *testing.T) {
			var body map[string]any
			profileReads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/loom/v1/creators/me/marketListings/listing-1":
					profileReads++
					_, _ = fmt.Fprint(w, `{"id":"listing-1","publishedVersionId":"lv-1","category":"writing","value":{"targetAudience":"Authors","problemSolved":"Drafts","scenarios":["News"],"notSuitableFor":"Legal advice"}}`)
				case "/loom/v1/creators/me/marketListings/listing-1/versions":
					_, _ = fmt.Fprint(w, `{"items":[{"id":"lv-1","sourceTemplateVersionId":"v1"}]}`)
				case "/loom/v1/marketListings":
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					_, _ = fmt.Fprint(w, `{}`)
				default:
					t.Errorf("unexpected URL %s", r.URL)
				}
			}))
			defer server.Close()
			cmd := newListingPublishCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
			args := []string{"template-1", "--listing-id", "listing-1", "--template-version-id", "v2", "--display-name", "Bot", "--task-fixed-fee", "0"}
			if explicitPackage {
				args = append(args, "--skill-package-archive-hash", "archive-1", "--skill-package-validation-id", "validation-1")
			}
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if profileReads != 1 {
				t.Fatalf("profileReads=%d", profileReads)
			}
			value := body["value"].(map[string]any)
			if value["targetAudience"] != "Authors" || value["notSuitableFor"] != "Legal advice" {
				t.Fatalf("lost public value %v", value)
			}
			if _, ok := body["category"]; ok {
				t.Fatalf("omitted category should preserve via Server, body=%v", body)
			}
		})
	}
}

func TestListingUpdatePartialProfilePreservesSiblingDetails(t *testing.T) {
	for _, tt := range []struct {
		name      string
		flags     []string
		wantValue bool
	}{
		{"category only", []string{"--category", "new_code"}, false},
		{"partial value", []string{"--problem-solved", "New problem"}, true},
		{"clear scenarios", []string{"--scenario", ""}, true},
		{"value with both name fields", []string{"--display-name", "New name", "--description", "New description", "--target-audience", ""}, true},
		{"summary only", []string{"--update-summary", "New summary"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/loom/v1/creators/me/marketListings/listing-1" {
					_, _ = fmt.Fprint(w, `{"displayName":"Old name","description":"Old description","category":"old_code","value":{"targetAudience":"Authors","problemSolved":"Drafts","scenarios":["News"],"notSuitableFor":"Legal advice"}}`)
					return
				}
				if r.URL.Path != "/loom/v1/marketListings/listing-1:updatePublicProfile" || r.Method != http.MethodPost {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				_, _ = fmt.Fprint(w, `{"id":"review-1","status":"pending"}`)
			}))
			defer server.Close()
			cmd := newListingUpdateCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetArgs(append([]string{"listing-1"}, tt.flags...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if tt.name != "value with both name fields" && (body["displayName"] != "Old name" || body["description"] != "Old description") {
				t.Fatalf("lost name/description %v", body)
			}
			value, hasValue := body["value"].(map[string]any)
			if hasValue != tt.wantValue {
				t.Fatalf("value present=%t, body=%v", hasValue, body)
			}
			if hasValue {
				if value["notSuitableFor"] != "Legal advice" {
					t.Fatalf("lost sibling field %v", value)
				}
				if tt.name == "clear scenarios" && !reflect.DeepEqual(value["scenarios"], []any{}) {
					t.Fatalf("clear scenarios=%v", value)
				}
				if tt.name == "partial value" && (value["targetAudience"] != "Authors" || value["problemSolved"] != "New problem") {
					t.Fatalf("value=%v", value)
				}
			}
			if tt.name == "category only" {
				if body["category"] != "new_code" {
					t.Fatalf("body=%v", body)
				}
			} else if _, ok := body["category"]; ok {
				t.Fatalf("omitted category must not be resubmitted: %v", body)
			}
		})
	}
}

func TestListingInvalidCategoryFailsBeforeAnyRemoteAction(t *testing.T) {
	for _, value := range []string{"", "写作", "Writing", "has-hyphen", strings.Repeat("a", 33)} {
		t.Run(value, func(t *testing.T) {
			for _, publish := range []bool{false, true} {
				opts := &rootOptions{server: "https://example.invalid/loom/v1", timeout: time.Second}
				cmd := newListingUpdateCmd(opts)
				args := []string{"listing-1", "--category", value}
				if publish {
					cmd = newListingPublishCmd(opts)
					args = []string{"template-1", "--template-version-id", "v1", "--display-name", "Bot", "--task-fixed-fee", "0", "--category", value}
				}
				cmd.SetOut(new(bytes.Buffer))
				cmd.SetErr(new(bytes.Buffer))
				cmd.SetArgs(args)
				if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--category") {
					t.Fatalf("error=%v", err)
				}
			}
		})
	}
}

func TestListingDetailDisplaysEffectiveCategoryAndUnlistedReason(t *testing.T) {
	var out bytes.Buffer
	pin := flexInt64(3)
	if err := printCreatorListingDetail(&out, creatorMarketListingResponse{ID: "listing-1", Category: "new_code", PinPosition: &pin, UnlistedReason: "Policy issue", Value: marketPublicValue{TargetAudience: "Authors"}}); err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, out.String(), "category", "new_code", "pin_position", "3", "unlisted_reason", "Policy issue", "Authors")
}

func TestListingProfileReadFailureDoesNotSubmitEmptyDetails(t *testing.T) {
	for _, publish := range []bool{false, true} {
		t.Run(fmt.Sprint(publish), func(t *testing.T) {
			posted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posted = true
					t.Error("profile read failure must stop submission")
				}
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = fmt.Fprint(w, `{"error":"profile unavailable"}`)
			}))
			defer server.Close()
			opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
			cmd := newListingUpdateCmd(opts)
			args := []string{"listing-1", "--problem-solved", "Updated problem"}
			if publish {
				cmd = newListingPublishCmd(opts)
				args = []string{"template-1", "--listing-id", "listing-1", "--template-version-id", "v1", "--display-name", "Bot", "--task-fixed-fee", "0", "--skill-package-archive-hash", "hash", "--skill-package-validation-id", "validation"}
			}
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "profile unavailable") {
				t.Fatalf("error=%v", err)
			}
			if posted {
				t.Fatal("submitted after failed profile read")
			}
		})
	}
}
