package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestModelListJSONDoesNotInventRemovedCatalogFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/models" || r.URL.Query().Get("stepType") != "video-generate" {
			t.Fatalf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"modelId":"ali/wan2.6-i2v","displayName":"Wan 2.6 I2V","supportedStepTypes":["video-generate"],"authoringOptions":[{"kind":"fixedModelContract","fixedModelContract":{"subjectRevisionId":"subject-1"}}]}]}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newModelListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--step-type", "video-generate"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("model list error = %v", err)
	}
	for _, want := range []string{"ali/wan2.6-i2v", "fixedModelContract", "subject-1"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q: %s", want, out.String())
		}
	}
	for _, forbidden := range []string{"available", "availabilityReason", "provider", "executionAdapter", "supportedApis", "isDefault"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("output must not invent removed raw catalog field %q: %s", forbidden, out.String())
		}
	}
}
