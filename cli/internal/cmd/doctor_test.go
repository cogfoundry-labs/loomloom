package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/platform"
)

func TestDoctorProbesProductAPIExecutables(t *testing.T) {
	requests := map[string]string{}
	var authorization string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests[r.URL.Path] = r.URL.RawQuery
		switch r.URL.Path {
		case "/loom/v1/marketListings":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/loom/v1/users/me/executables":
			authorization = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/release":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v0.0.1"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("LOOMLOOM_CLI_RELEASE_API", server.URL+"/release")

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		token:   "token-1",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newDoctorCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command error = %v", err)
	}
	if _, called := requests["/version"]; called {
		t.Fatalf("doctor should not call /version, but it did")
	}
	if requests["/loom/v1/marketListings"] != "pageSize=1" {
		t.Fatalf("market query=%q want pageSize=1", requests["/loom/v1/marketListings"])
	}
	if requests["/loom/v1/users/me/executables"] != "pageSize=1" {
		t.Fatalf("executables query=%q want pageSize=1", requests["/loom/v1/users/me/executables"])
	}
	if authorization != "Bearer token-1" {
		t.Fatalf("authorization=%q want Bearer token-1", authorization)
	}
	if !strings.Contains(out.String(), `"healthy": true`) {
		t.Fatalf("output=%s want healthy true", out.String())
	}
	if !strings.Contains(out.String(), `"message": "ok"`) {
		t.Fatalf("output=%s want message ok", out.String())
	}
}

func TestDoctorWithoutTokenAndUnknownPlatformShowsChoosePlatform(t *testing.T) {
	isolateCmdConfigHome(t)
	opts := &rootOptions{
		server:  "",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newDoctorCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command error = %v", err)
	}
	if !strings.Contains(out.String(), `"credential_action": "choose_platform"`) {
		t.Fatalf("output=%s want choose_platform", out.String())
	}
	for _, want := range []string{
		"你还没有配置 LoomLoom 密钥",
		"https://console.shengsuanyun.com/user/keys",
		"CogFoundry：面向新加坡及其他海外地区用户，当前支付和交易能力敬请期待。",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s want %q", out.String(), want)
		}
	}
}

func TestDoctorWithoutTokenAndBoundShengSuanYunShowsMissingToken(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{Platform: platform.ShengSuanYun}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	opts := &rootOptions{
		server:  "http://127.0.0.1:8080/loom/v1",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newDoctorCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command error = %v", err)
	}
	if !strings.Contains(out.String(), `"credential_action": "missing_token"`) {
		t.Fatalf("output=%s want missing_token", out.String())
	}
	for _, want := range []string{
		"当前未检测到胜算云密钥",
		"https://console.shengsuanyun.com/user/keys",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output=%s want %q", out.String(), want)
		}
	}
}

func TestDoctorCogFoundryReturnsStructuredUnavailableMessage(t *testing.T) {
	isolateCmdConfigHome(t)
	opts := &rootOptions{
		server:  "https://api.cogfoundry.ai/loom/v1",
		token:   "token-1",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newDoctorCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command error = %v", err)
	}
	if !strings.Contains(out.String(), `"credential_action": "cogfoundry_unavailable"`) {
		t.Fatalf("output=%s want cogfoundry_unavailable", out.String())
	}
	if !strings.Contains(out.String(), cogFoundryUnavailableMessage) {
		t.Fatalf("output=%s want fixed CogFoundry unavailable message", out.String())
	}
}

func TestDoctorAuthenticationFailureUsesPlatformCredentialMessage(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{Platform: platform.ShengSuanYun}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loom/v1/marketListings":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/loom/v1/users/me/executables":
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		token:   "bad-token",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newDoctorCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command error = %v", err)
	}
	if !strings.Contains(out.String(), `"credential_action": "missing_token"`) {
		t.Fatalf("output=%s want missing_token", out.String())
	}
	if !strings.Contains(out.String(), "当前未检测到胜算云密钥") {
		t.Fatalf("output=%s want fixed ShengSuanYun token message", out.String())
	}
}

func TestDoctorSuccessfulAuthenticatedProbePersistsPlatform(t *testing.T) {
	isolateCmdConfigHome(t)
	opts := &rootOptions{
		server: "https://loomloom-test.shengsuanyun.com/loom/v1",
		token:  "token-1",
	}
	maybePersistVerifiedPlatform(opts, true)
	got := platform.LoadState()
	if got.Platform != platform.ShengSuanYun {
		t.Fatalf("platform=%q want %q", got.Platform, platform.ShengSuanYun)
	}
}

func TestDoctorFailedProductProbeDoesNotPersistPlatform(t *testing.T) {
	isolateCmdConfigHome(t)
	opts := &rootOptions{
		server: "https://loomloom323.shengsuanyun.com/loom/v1",
		token:  "token-1",
	}
	maybePersistVerifiedPlatform(opts, false)
	got := platform.LoadState()
	if got.Platform != "" {
		t.Fatalf("platform=%q want empty", got.Platform)
	}
}

func TestDoctorDecodeFailureDoesNotPersistPlatformThroughCallback(t *testing.T) {
	isolateCmdConfigHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loom/v1/marketListings":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/loom/v1/users/me/executables":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`not-json`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		token:   "token-1",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newDoctorCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "decode response JSON") {
		t.Fatalf("error=%v want decode response JSON", err)
	}
	got := platform.LoadState()
	if got.Platform != "" {
		t.Fatalf("platform=%q want empty after decode failure", got.Platform)
	}
}

func TestDoctorSuppressesReleaseCheckErrorsInJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loom/v1/marketListings":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/loom/v1/users/me/executables":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/release":
			http.NotFound(w, r)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("LOOMLOOM_CLI_RELEASE_API", server.URL+"/release")

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		token:   "token-1",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newDoctorCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command error = %v", err)
	}
	if strings.Contains(out.String(), "version_check_error") {
		t.Fatalf("output=%s should not include release check error", out.String())
	}
	if !strings.Contains(out.String(), `"healthy": true`) {
		t.Fatalf("output=%s want healthy true", out.String())
	}
}
