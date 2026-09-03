package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestListingUnlistUsesUnlistEndpointWithoutCreatorUserID(t *testing.T) {
	var requestedPath string
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		requestedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"listing-1","sale_status":"unlisted"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingUnlistCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing unlist command error = %v", err)
	}
	if requestedPath != "/loom/v1/marketListings/listing-1:unlist" {
		t.Fatalf("path=%q want unlist endpoint", requestedPath)
	}
	if requestedQuery != "" {
		t.Fatalf("query=%q want no identity query", requestedQuery)
	}
	if !strings.Contains(out.String(), `"sale_status": "unlisted"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestListingUpdateDescriptionPreservesCurrentDisplayName(t *testing.T) {
	var updateBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/creators/me/marketListings/listing-1":
			_, _ = w.Write([]byte(`{"displayName":"Current name","description":"Old description"}`))
		case "/loom/v1/marketListings/listing-1:updatePublicProfile":
			if err := json.NewDecoder(r.Body).Decode(&updateBody); err != nil {
				t.Fatalf("decode update body: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":"review-1","status":"pending"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingUpdateCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--description", "New description"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing update command error = %v", err)
	}
	if updateBody["displayName"] != "Current name" {
		t.Fatalf("displayName=%v want Current name", updateBody["displayName"])
	}
	if updateBody["description"] != "New description" {
		t.Fatalf("description=%v want New description", updateBody["description"])
	}
}

func TestListingUpdateDisplayNamePreservesCurrentDescription(t *testing.T) {
	var updateBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/creators/me/marketListings/listing-1":
			_, _ = w.Write([]byte(`{"displayName":"Current name","description":"Current description"}`))
		case "/loom/v1/marketListings/listing-1:updatePublicProfile":
			if err := json.NewDecoder(r.Body).Decode(&updateBody); err != nil {
				t.Fatalf("decode update body: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":"review-1","status":"pending"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingUpdateCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--display-name", "New name"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing update command error = %v", err)
	}
	if updateBody["displayName"] != "New name" {
		t.Fatalf("displayName=%v want New name", updateBody["displayName"])
	}
	if updateBody["description"] != "Current description" {
		t.Fatalf("description=%v want Current description", updateBody["description"])
	}
}

func TestListingUpdateWithBothFieldsSkipsCurrentProfileLookup(t *testing.T) {
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"review-1","status":"pending"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingUpdateCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--display-name", "New name", "--description", "New description"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing update command error = %v", err)
	}
	if len(requestedPaths) != 1 || requestedPaths[0] != "/loom/v1/marketListings/listing-1:updatePublicProfile" {
		t.Fatalf("requested paths=%v want only update endpoint", requestedPaths)
	}
}

func TestListingUpdateSkillPackageUsesArchiveSelectionAndRequestID(t *testing.T) {
	var requestedPath string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode update-skill-package body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"review-1","status":"pending"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingUpdateSkillPackageCmd(opts)
	cmd.SetArgs([]string{
		"listing-1", "--request-id", "request-1",
		"--skill-package-archive-hash", "sha256:archive",
		"--skill-package-validation-id", "validation-1",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update-skill-package command error = %v", err)
	}
	if requestedPath != "/loom/v1/creators/me/marketListings/listing-1:updateSkillPackage" {
		t.Fatalf("path=%q want update-skill-package endpoint", requestedPath)
	}
	if body["requestId"] != "request-1" {
		t.Fatalf("requestId=%v want request-1", body["requestId"])
	}
	selection, ok := body["skillPackage"].(map[string]any)
	if !ok || selection["mode"] != "archive" || selection["expectedArchiveHash"] != "sha256:archive" || selection["expectedValidationId"] != "validation-1" {
		t.Fatalf("unexpected skillPackage=%#v", body["skillPackage"])
	}
}

func TestListingUpdateSkillPackageUsesAutoWhenTupleIsOmitted(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode update-skill-package body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"review-1","status":"pending"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingUpdateSkillPackageCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--request-id", "request-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update-skill-package command error = %v", err)
	}
	selection, ok := body["skillPackage"].(map[string]any)
	if !ok || selection["mode"] != "auto" {
		t.Fatalf("skillPackage=%#v want auto", body["skillPackage"])
	}
	if _, ok := selection["expectedArchiveHash"]; ok {
		t.Fatalf("auto selection should not include expectedArchiveHash: %#v", selection)
	}
	if _, ok := selection["expectedValidationId"]; ok {
		t.Fatalf("auto selection should not include expectedValidationId: %#v", selection)
	}
}

func TestListingUpdateSkillPackageExplainsListingNotPublished(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_argument","message":"listing_not_published"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingUpdateSkillPackageCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--request-id", "request-1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("update-skill-package command unexpectedly succeeded")
	}
	assertContainsAll(t, err.Error(),
		"listing_not_published",
		"only applies to an already published listing",
		"bind the ZIP during the initial listing publish",
	)
}

func TestListingUpdateRejectsMissingCurrentDisplayName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"displayName":"","description":"Current description"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingUpdateCmd(opts)
	cmd.SetArgs([]string{"listing-1", "--description", "New description"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "display name is required") {
		t.Fatalf("error=%v want missing display name error", err)
	}
}

func TestListingWithdrawResolvesPendingReviewAndPassesReason(t *testing.T) {
	var requestedQuery string
	var withdrawBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/creators/me/marketReviewRequests":
			requestedQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"items":[
				{"id":"review-other","listingId":"listing-other","status":"pending"},
				{"id":"review-1","listingId":"listing-1","status":"pending"}
			]}`))
		case "/loom/v1/creators/me/marketReviewRequests/review-1:withdraw":
			if err := json.NewDecoder(r.Body).Decode(&withdrawBody); err != nil {
				t.Fatalf("decode withdraw body: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":"review-1","status":"withdrawn"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingWithdrawCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1", "--reason", "cancelled by creator"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing withdraw command error = %v", err)
	}
	if requestedQuery != "pageSize=500&status=pending" {
		t.Fatalf("query=%q want pending reviews with page size 500", requestedQuery)
	}
	if withdrawBody["reason"] != "cancelled by creator" {
		t.Fatalf("reason=%v want cancelled by creator", withdrawBody["reason"])
	}
	if !strings.Contains(out.String(), `"status": "withdrawn"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestListingWithdrawVerboseLogsDescribeBothSteps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/creators/me/marketReviewRequests":
			_, _ = w.Write([]byte(`{"items":[{"id":"review-1","listingId":"listing-1","status":"pending"}]}`))
		case "/loom/v1/creators/me/marketReviewRequests/review-1:withdraw":
			_, _ = w.Write([]byte(`{"id":"review-1","status":"withdrawn"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	var logs bytes.Buffer
	opts := &rootOptions{
		server:    server.URL + "/loom/v1",
		timeout:   time.Second,
		verbose:   true,
		logWriter: &logs,
	}
	cmd := newListingWithdrawCmd(opts)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing withdraw command error = %v", err)
	}
	for _, want := range []string{
		"resolving pending review listing_id=listing-1",
		"pending review resolved review_request_id=review-1",
		"review withdrawn review_request_id=review-1",
		"GET /loom/v1/creators/me/marketReviewRequests",
		"POST /loom/v1/creators/me/marketReviewRequests/review-1:withdraw",
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("logs=%q want %q", logs.String(), want)
		}
	}
}

func TestListingWithdrawReportsListFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"temporary failure"}`, http.StatusServiceUnavailable)
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingWithdrawCmd(opts)
	cmd.SetArgs([]string{"listing-1"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "list pending review requests") {
		t.Fatalf("error=%v want list failure context", err)
	}
}

func TestListingWithdrawReportsWithdrawFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/loom/v1/creators/me/marketReviewRequests":
			_, _ = w.Write([]byte(`{"items":[{"id":"review-1","listingId":"listing-1","status":"pending"}]}`))
		case "/loom/v1/creators/me/marketReviewRequests/review-1:withdraw":
			http.Error(w, `{"error":"conflict"}`, http.StatusConflict)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingWithdrawCmd(opts)
	cmd.SetArgs([]string{"listing-1"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "withdraw pending review request review-1") {
		t.Fatalf("error=%v want withdraw failure context", err)
	}
}

func TestListingWithdrawRejectsMissingPendingReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingWithdrawCmd(opts)
	cmd.SetArgs([]string{"listing-1"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "has no pending review request") {
		t.Fatalf("error=%v want no pending review error", err)
	}
}

func TestListingWithdrawRejectsMultiplePendingReviews(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[
			{"id":"review-2","listingId":"listing-1","status":"pending"},
			{"id":"review-1","listingId":"listing-1","status":"pending"}
		]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingWithdrawCmd(opts)
	cmd.SetArgs([]string{"listing-1"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "review-1, review-2") {
		t.Fatalf("error=%v want sorted conflicting review IDs", err)
	}
}

func TestListingListTextShowsFormattedFee(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{
			"id":"listing-1",
			"displayName":"Writer",
			"status":"active",
			"listingVersionId":"lv-1",
			"reviewStatus":"approved",
			"taskFixedFeeT":5000000,
			"taskFixedFee":{"amount":"0.5000000","currency":"CNY"},
			"currency":"CNY",
			"saleStatus":"on_sale",
			"executionAvailabilityStatus":"available"
		}]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing list command error = %v", err)
	}
	assertContainsAll(t, out.String(), "CNY 0.5", "approved", "on_sale", "available")
	assertContainsNone(t, out.String(), "task_fixed_fee_t")
	if strings.Contains(out.String(), "{") {
		t.Fatalf("output=%s must not be raw JSON", out.String())
	}
}

func TestListingListTextUnknownCurrencyFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"listing-1","taskFixedFeeT":5000000}]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing list command error = %v", err)
	}
	for _, want := range []string{"(currency unknown) 5000000", "5000000"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
}

func TestListingListJSONPreservesUnknownFieldsWithoutRawT(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"listing-1","taskFixedFeeT":5000000,"taskFixedFee":{"amount":"0.5000000","currency":"CNY"},"newBackendField":"kept"}]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newListingListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing list command error = %v", err)
	}
	for _, want := range []string{`"taskFixedFee": {`, `"amount": "0.5"`, `"newBackendField": "kept"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
	assertContainsNone(t, out.String(), "taskFixedFeeT")
}

func TestListingShowTextShowsFormattedFee(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"listing-1",
			"displayName":"Writer",
			"status":"active",
			"reviewRequestId":"review-1",
			"reviewStatus":"pending_review",
			"taskFixedFeeT":5000000,
			"currency":"CNY",
			"saleStatus":"unlisted",
			"skillPackage":{"available":false,"unavailableReason":"listing_not_listed"},
			"skillPackageReview":{"pending":{
				"id":"skill-package-version-1",
				"mode":"archive",
				"status":"pending",
				"archiveHash":"sha256:archive",
				"validationId":"validation-1",
				"sizeBytes":7875
			}}
		}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingShowCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing show command error = %v", err)
	}
	assertContainsAll(t, out.String(),
		"CNY 0.5",
		"pending_review",
		"skill_package_review_binding", "bound",
		"skill_package_review_version_id", "skill-package-version-1",
		"skill_package_review_archive_hash", "sha256:archive",
		"skill_package_public_available", "false",
		"skill_package_public_unavailable_reason", "listing_not_listed",
	)
	assertContainsNone(t, out.String(), "task_fixed_fee_t")
}

func TestListingShowTextDoesNotTreatMissingReviewViewAsUnbound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"listing-1",
			"displayName":"Writer",
			"reviewStatus":"pending_review",
			"currency":"CNY",
			"skillPackage":{"available":false,"unavailableReason":"listing_not_listed"}
		}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingShowCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing show command error = %v", err)
	}
	assertContainsAll(t, out.String(), "skill_package_review_binding", "unknown_from_current_server")
	assertContainsNone(t, out.String(), "unbound")
}

func TestListingVersionsTextShowsFormattedFee(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{
			"id":"lv-1",
			"versionNumber":1,
			"status":"active",
			"saleStatus":"on_sale",
			"reviewStatus":"approved",
			"taskFixedFeeT":5000000,
			"currency":"CNY",
			"createdAtUnix":1700000000,
			"executionAvailabilityStatus":"available"
		}]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingVersionsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing versions command error = %v", err)
	}
	for _, want := range []string{"CNY 0.5", "lv-1", "approved", "available"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
}

func TestListingVersionsTextShowsBlockedAndReviewNotes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{
			"id":"lv-1",
			"versionNumber":1,
			"status":"rejected",
			"saleStatus":"unlisted",
			"reviewStatus":"rejected",
			"reviewReason":"input schema missing required field",
			"taskFixedFeeT":5000000,
			"createdAtUnix":1700000000,
			"executionAvailabilityStatus":"blocked",
			"executionBlockReason":"source template version was withdrawn"
		}]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}
	cmd := newListingVersionsCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"listing-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("listing versions command error = %v", err)
	}
	for _, want := range []string{
		"blocked",
		"source template version was withdrawn",
		"input schema missing required field",
		"notes:",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s missing %q", out.String(), want)
		}
	}
}
