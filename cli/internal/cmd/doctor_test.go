package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cogfoundry-labs/loomloom/cli/internal/platform"
)

func healthyDoctorServer(t *testing.T, authenticatedStatus int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loom/v1/marketListings":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/loom/v1/users/me/executables":
			if authenticatedStatus != 0 && authenticatedStatus != http.StatusOK {
				http.Error(w, `{"error":"unauthorized"}`, authenticatedStatus)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/release":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v0.0.1"}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func executeDoctorJSON(t *testing.T, opts *rootOptions, args ...string) map[string]any {
	t.Helper()
	opts.output = "json"
	if opts.timeout == 0 {
		opts.timeout = time.Second
	}
	cmd := newDoctorCmd(opts)
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor error=%v output=%s", err, out.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output=%q error=%v", out.String(), err)
	}
	return payload
}

func TestDoctorSuccessPersistsCustomProfileAndReturnsTokenBinding(t *testing.T) {
	isolateCmdConfigHome(t)
	server := healthyDoctorServer(t, http.StatusOK)
	defer server.Close()
	t.Setenv("LOOMLOOM_CLI_RELEASE_API", server.URL+"/release")

	payload := executeDoctorJSON(t, &rootOptions{
		server: server.URL + "/loom/v1",
		token:  "token-1",
	})
	for key, want := range map[string]any{
		"platform":        "custom",
		"platform_preset": false,
		"healthy":         true,
		"token_set":       true,
		"token_valid":     true,
		"next_action":     "persist_token",
	} {
		if got := payload[key]; got != want {
			t.Fatalf("%s=%v want %v payload=%v", key, got, want, payload)
		}
	}
	profileName, _ := payload["profile"].(string)
	tokenEnv, _ := payload["token_env"].(string)
	if profileName == "" || tokenEnv != platform.TokenEnvName(profileName) {
		t.Fatalf("profile=%q token_env=%q want generated binding", profileName, tokenEnv)
	}
	state := platform.LoadState()
	active, ok := state.ActiveProfile()
	if !ok || active.Server != server.URL+"/loom/v1" || active.TokenEnv != tokenEnv {
		t.Fatalf("state=%+v want persisted active profile", state)
	}
}

func TestDoctorReturnsNoneWhenDedicatedTokenEnvironmentAlreadySet(t *testing.T) {
	isolateCmdConfigHome(t)
	server := healthyDoctorServer(t, http.StatusOK)
	defer server.Close()
	t.Setenv("LOOMLOOM_CLI_RELEASE_API", server.URL+"/release")
	name, err := platform.GenerateProfileName(server.URL+"/loom/v1", platform.Custom, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(platform.TokenEnvName(name), "token-1")

	payload := executeDoctorJSON(t, &rootOptions{
		server: server.URL + "/loom/v1",
		token:  "token-1",
	})
	if payload["next_action"] != "none" {
		t.Fatalf("payload=%v want next_action none", payload)
	}
}

func TestDoctorWithoutServerChoosesPlatform(t *testing.T) {
	isolateCmdConfigHome(t)
	payload := executeDoctorJSON(t, &rootOptions{token: "token-1"})
	if payload["credential_action"] != "choose_platform" || payload["next_action"] != "choose_server" {
		t.Fatalf("payload=%v want platform selection", payload)
	}
	message, _ := payload["credential_message"].(string)
	if !strings.Contains(message, "胜算云") ||
		!strings.Contains(message, "CogFoundry") ||
		!strings.Contains(message, "https://loomloom.shengsuanyun.com/loom/v1") ||
		!strings.Contains(message, "https://console.shengsuanyun.com/user/keys") ||
		!strings.Contains(message, "https://console.shengsuanyun.com/user/recharge") ||
		!strings.Contains(message, "https://loomloom.cogfoundry.ai/loom/v1") ||
		!strings.Contains(message, "https://console.cogfoundry.ai/api-keys") ||
		!strings.Contains(message, "https://console.cogfoundry.ai/credits") {
		t.Fatalf("payload=%v want both preset platforms", payload)
	}
	if strings.Contains(message, "相关地址未知") || strings.Contains(message, "当前环境提供") {
		t.Fatalf("payload=%v must not report known CogFoundry configuration as unknown", payload)
	}
}

func TestDoctorCogFoundryWithoutTokenDoesNotReportUnavailable(t *testing.T) {
	isolateCmdConfigHome(t)
	payload := executeDoctorJSON(t, &rootOptions{
		server: "https://api.cogfoundry.ai/loom/v1",
	})
	if payload["credential_action"] != "missing_token" || payload["next_action"] != "configure_token" {
		t.Fatalf("payload=%v want missing CogFoundry token", payload)
	}
	if payload["platform_preset"] != true {
		t.Fatalf("payload=%v want CogFoundry preset", payload)
	}
	message, _ := payload["credential_message"].(string)
	if !strings.Contains(message, "https://console.cogfoundry.ai/api-keys") {
		t.Fatalf("payload=%v want CogFoundry API key guidance", payload)
	}
	if strings.Contains(strings.ToLower(message), "unavailable") || strings.Contains(message, "当前环境对应") {
		t.Fatalf("payload=%v must not report CogFoundry unavailable", payload)
	}
}

func TestDoctorAuthenticationFailureDoesNotPersist(t *testing.T) {
	isolateCmdConfigHome(t)
	server := healthyDoctorServer(t, http.StatusUnauthorized)
	defer server.Close()

	payload := executeDoctorJSON(t, &rootOptions{
		server: server.URL + "/loom/v1",
		token:  "bad-token",
	})
	if payload["token_valid"] != false || payload["next_action"] != "replace_token" {
		t.Fatalf("payload=%v want invalid token result", payload)
	}
	message, _ := payload["credential_message"].(string)
	if !strings.Contains(message, "密钥认证未通过") || strings.Contains(message, "平台不一致") {
		t.Fatalf("payload=%v want neutral authentication failure message", payload)
	}
	if got := platform.LoadState(); len(got.Servers) != 0 {
		t.Fatalf("state=%+v want no persistence", got)
	}
}

func TestDoctorProbeFailureIsStructuredAndDoesNotPersist(t *testing.T) {
	isolateCmdConfigHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	payload := executeDoctorJSON(t, &rootOptions{
		server: server.URL + "/loom/v1",
		token:  "token-1",
	})
	if payload["healthy"] != false || payload["next_action"] != "fix_server" {
		t.Fatalf("payload=%v want fix_server", payload)
	}
	if got := platform.LoadState(); len(got.Servers) != 0 {
		t.Fatalf("state=%+v want no persistence", got)
	}
}

func TestDoctorFailureDoesNotEchoTokenFromServerResponse(t *testing.T) {
	isolateCmdConfigHome(t)
	const token = "doctor-secret-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "reflected Authorization: Bearer "+token, http.StatusInternalServerError)
	}))
	defer server.Close()

	payload := executeDoctorJSON(t, &rootOptions{
		server: server.URL + "/loom/v1",
		token:  token,
	})
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), token) {
		t.Fatalf("JSON Doctor output leaked token: %s", encoded)
	}
	if payload["credential_message"] != "server returned HTTP 500" {
		t.Fatalf("payload=%v want safe HTTP detail", payload)
	}

	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		token:   token,
		timeout: time.Second,
		output:  "text",
	}
	cmd := newDoctorCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("text Doctor error=%v output=%s", err, out.String())
	}
	if strings.Contains(out.String(), token) {
		t.Fatalf("text Doctor output leaked token: %s", out.String())
	}
}

func TestDoctorSupportsExplicitProfileName(t *testing.T) {
	isolateCmdConfigHome(t)
	server := healthyDoctorServer(t, http.StatusOK)
	defer server.Close()
	t.Setenv("LOOMLOOM_CLI_RELEASE_API", server.URL+"/release")

	payload := executeDoctorJSON(t, &rootOptions{
		server: server.URL + "/loom/v1",
		token:  "token-1",
	}, "--name", "local-test")
	if payload["profile"] != "local-test" || payload["token_env"] != "LOOMLOOM_TOKEN_LOCAL_TEST" {
		t.Fatalf("payload=%v want explicit profile binding", payload)
	}
}
