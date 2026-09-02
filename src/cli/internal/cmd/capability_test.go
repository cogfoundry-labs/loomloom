package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCapabilityResolveJSONUsesServerAuthoritativeEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/authoringCapabilities:resolve" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query()["inputModality"]; len(got) != 2 || got[0] != "image" || got[1] != "text" {
			t.Fatalf("inputModality = %#v", got)
		}
		if r.URL.Query().Get("outputModality") != "video" || r.URL.Query().Get("modelId") != "ali/wan2.6-i2v" {
			t.Fatalf("unexpected query %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schemaVersion":"loomloom-authoring-capability-resolution/v1","status":"supported","query":{"inputModalities":["image","text"],"outputModality":"video","modelId":"ali/wan2.6-i2v"},"matches":[{"authoringKind":"fixedModelContract","operation":"image-to-video","stepType":"video-generate","inputModalities":["image","text"],"requiredInputModalities":["image"],"outputModalities":["video"],"contract":{"subjectRevisionId":"subject-video-v1"},"eligibleModels":[{"modelId":"ali/wan2.6-i2v","authoringOptions":[]}]}],"nextAction":"author_template_spec"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"}
	cmd := newCapabilityResolveCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--input", "image", "--input", "text", "--output-modality", "video", "--model-id", "ali/wan2.6-i2v"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("capability resolve error = %v", err)
	}
	for _, want := range []string{"loomloom-authoring-capability-resolution/v1", "subject-video-v1", "ali/wan2.6-i2v", "requiredInputModalities"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q: %s", want, out.String())
		}
	}
}

func TestCapabilityResolveTextShowsProfileAuthority(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"supported","matches":[{"authoringKind":"capabilityProfile","operation":"image-to-text","stepType":"text-generate","profile":{"profileId":"text.vision.openai-chat.v1","revision":"2026-08-25.1"},"eligibleModels":[{"modelId":"google/gemini-3-flash"}]}],"nextAction":"author_template_spec"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "text"}
	cmd := newCapabilityResolveCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--input", "image", "--output-modality", "text"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("capability resolve error = %v", err)
	}
	for _, want := range []string{"image-to-text", "capabilityProfile", "text.vision.openai-chat.v1@2026-08-25.1", "google/gemini-3-flash"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q: %s", want, out.String())
		}
	}
}

func TestCapabilityResolveTextShowsDynamicProfileWithoutEmptyRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"supported","matches":[{"authoringKind":"capabilityProfile","operation":"text-to-image","stepType":"image-generate","profile":{"profileId":"image.text-to-image.v1","dynamic":true},"eligibleModels":[{"modelId":"ali/qwen-image-plus"}]}],"nextAction":"author_template_spec"}`))
	}))
	defer server.Close()

	opts := &rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "text"}
	cmd := newCapabilityResolveCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--input", "text", "--output-modality", "image"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("capability resolve error = %v", err)
	}
	for _, want := range []string{"text-to-image", "capabilityProfile", "image.text-to-image.v1", "ali/qwen-image-plus"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q: %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "image.text-to-image.v1@") {
		t.Fatalf("dynamic Profile authority contains an empty revision suffix: %s", out.String())
	}
}

func TestCapabilityResolveRequiresInputAndOutput(t *testing.T) {
	opts := &rootOptions{timeout: time.Second}
	cmd := newCapabilityResolveCmd(opts)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--input") {
		t.Fatalf("error = %v, want missing input", err)
	}

	cmd = newCapabilityResolveCmd(opts)
	cmd.SetArgs([]string{"--input", "image"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--output-modality") {
		t.Fatalf("error = %v, want missing output", err)
	}
}
