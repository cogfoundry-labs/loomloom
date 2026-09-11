package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSubscriptionBuyRequiresConfirmationBeforePost(t *testing.T) {
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
			t.Fatalf("preview must not submit a purchase")
		}
		if r.URL.Path != "/loom/v1/subscriptionBundles/bundle-1" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"bundle-1","revision":3,"tiers":[{"tierType":"yearly","durationDays":366}]}`))
	}))
	defer server.Close()

	command := newSubscriptionBuyCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"bundle-1", "--tier", "yearly"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute preview: %v", err)
	}
	if postCount != 0 {
		t.Fatalf("post count = %d", postCount)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"confirmed": false`)) || !bytes.Contains(output.Bytes(), []byte(`"durationDays": 366`)) {
		t.Fatalf("unexpected preview: %s", output.String())
	}
}

func TestSubscriptionBuyPostsStableIdempotencyAndRevision(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/loom/v1/users/me/subscriptions" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"subscription-1","state":"active"}`))
	}))
	defer server.Close()

	command := newSubscriptionBuyCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	command.SetOut(&bytes.Buffer{})
	command.SetArgs([]string{
		"bundle-1", "--tier", "yearly", "--confirm",
		"--expected-revision", "3", "--client-request-id", "purchase-1",
	})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute purchase: %v", err)
	}
	if body["clientRequestId"] != "purchase-1" || body["expectedRevision"] != float64(3) ||
		body["tierType"] != "yearly" || body["bundleId"] != "bundle-1" {
		t.Fatalf("unexpected request body: %#v", body)
	}
}

func TestSubscriptionItemsPassesOpaquePagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/users/me/subscriptions/subscription-1/items" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("pageSize") != "7" || r.URL.Query().Get("pageToken") != "opaque-token" {
			t.Fatalf("query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"bundleRevision":2}`))
	}))
	defer server.Close()

	command := newSubscriptionItemsCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	command.SetOut(&bytes.Buffer{})
	command.SetArgs([]string{"subscription-1", "--page-size", "7", "--page-token", "opaque-token"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute items: %v", err)
	}
}

func TestMarketBundleItemsPassesOpaquePagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/subscriptionBundles/bundle-1/items" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("pageSize") != "11" || r.URL.Query().Get("pageToken") != "signed-token" {
			t.Fatalf("query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"bundleRevision":4}`))
	}))
	defer server.Close()

	command := newMarketBundleItemsCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
	command.SetOut(&bytes.Buffer{})
	command.SetArgs([]string{"bundle-1", "--page-size", "11", "--page-token", "signed-token"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute bundle items: %v", err)
	}
}
