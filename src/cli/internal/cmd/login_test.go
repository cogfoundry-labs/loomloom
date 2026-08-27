package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/authflow"
	"github.com/cogfoundry-labs/loomloom/src/cli/internal/platform"
)

func TestLoginHelpDistinguishesAuthorizationAndHTTPTimeouts(t *testing.T) {
	isolateCmdConfigHome(t)
	var output bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"login", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("login help error = %v", err)
	}
	for _, want := range []string{
		"--login-timeout duration",
		"Maximum time to wait for browser authorization",
		"default 5m0s",
		"--timeout duration",
		"HTTP request timeout",
		"default 30s",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("login help missing %q:\n%s", want, output.String())
		}
	}
}

func TestLoginRejectsNonPositiveTimeouts(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "zero login timeout",
			args:    []string{"--login-timeout", "0s"},
			wantErr: "--login-timeout must be greater than 0",
		},
		{
			name:    "negative login timeout",
			args:    []string{"--login-timeout", "-1s"},
			wantErr: "--login-timeout must be greater than 0",
		},
		{
			name:    "zero HTTP timeout",
			args:    []string{"--timeout", "0s"},
			wantErr: "--timeout must be greater than 0",
		},
		{
			name:    "negative HTTP timeout",
			args:    []string{"--timeout", "-1s"},
			wantErr: "--timeout must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateCmdConfigHome(t)
			cmd := NewRootCmd()
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			args := []string{"login", "--server", "https://loomloom.cogfoundry.ai/loom/v1"}
			args = append(args, tt.args...)
			cmd.SetArgs(args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v want %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoginPropagatesSeparateTimeoutsToAuthFlow(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantLogin    time.Duration
		wantExchange time.Duration
	}{
		{
			name:         "defaults",
			args:         []string{"--no-browser"},
			wantLogin:    authflow.DefaultAuthorizationTimeout,
			wantExchange: defaultHTTPTimeout,
		},
		{
			name:         "custom values",
			args:         []string{"--no-browser", "--login-timeout", "2m"},
			wantLogin:    2 * time.Minute,
			wantExchange: 250 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpTimeout := tt.wantExchange
			opts := &rootOptions{
				server:  "https://loomloom.cogfoundry.ai/loom/v1",
				timeout: httpTimeout,
				output:  "text",
			}
			var captured authflow.Config
			stop := errors.New("stop after capturing login config")
			runner := func(_ context.Context, cfg authflow.Config) (*authflow.Result, error) {
				captured = cfg
				return nil, stop
			}
			cmd := newLoginCmdWithRunner(opts, runner)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(tt.args)
			if err := cmd.Execute(); !errors.Is(err, stop) {
				t.Fatalf("login error = %v want capture sentinel", err)
			}
			if captured.AuthorizationTimeout != tt.wantLogin {
				t.Fatalf("authorization timeout = %s want %s", captured.AuthorizationTimeout, tt.wantLogin)
			}
			if captured.ExchangeTimeout != tt.wantExchange {
				t.Fatalf("exchange timeout = %s want %s", captured.ExchangeTimeout, tt.wantExchange)
			}
			if captured.CallbackPageVariant != authflow.CallbackPageCogFoundry {
				t.Fatalf("callback page variant = %q want %q", captured.CallbackPageVariant, authflow.CallbackPageCogFoundry)
			}
			if captured.OpenURL == nil {
				t.Fatal("--no-browser did not disable automatic browser opening")
			}
		})
	}
}

func TestLoginSelectsCallbackPageVariantForPlatform(t *testing.T) {
	tests := []struct {
		name   string
		server string
		want   authflow.CallbackPageVariant
	}{
		{
			name:   "CogFoundry",
			server: "https://loomloom.cogfoundry.ai/loom/v1",
			want:   authflow.CallbackPageCogFoundry,
		},
		{
			name:   "ShengSuanYun",
			server: "https://loomloom.shengsuanyun.com/loom/v1",
			want:   authflow.CallbackPageShengSuanYun,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &rootOptions{
				server:  tt.server,
				timeout: defaultHTTPTimeout,
				output:  "text",
			}
			var captured authflow.Config
			stop := errors.New("stop after capturing login config")
			runner := func(_ context.Context, cfg authflow.Config) (*authflow.Result, error) {
				captured = cfg
				return nil, stop
			}
			cmd := newLoginCmdWithRunner(opts, runner)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"--no-browser"})

			if err := cmd.Execute(); !errors.Is(err, stop) {
				t.Fatalf("login error = %v want capture sentinel", err)
			}
			if captured.CallbackPageVariant != tt.want {
				t.Fatalf("callback page variant = %q want %q", captured.CallbackPageVariant, tt.want)
			}
		})
	}
}

func TestCallbackPageVariantForUnknownPlatformUsesGenericFallback(t *testing.T) {
	for _, platformID := range []platform.ID{platform.Custom, platform.Unknown, platform.ID("future-platform")} {
		if got := callbackPageVariantForPlatform(platformID); got != authflow.CallbackPageGeneric {
			t.Errorf("callback page variant for %q = %q want generic", platformID, got)
		}
	}
}

func TestLoginRefusesPlatformWithoutBrowserLogin(t *testing.T) {
	isolateCmdConfigHome(t)
	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"login", "--server", "http://127.0.0.1:39999/loom/v1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "does not support browser login") {
		t.Fatalf("error = %v want browser login refusal", err)
	}
}

func TestLoginDoesNotRequireExistingTokenBinding(t *testing.T) {
	isolateCmdConfigHome(t)
	t.Setenv("LOOMLOOM_TOKEN", "unbound-token")
	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"login", "--server", "http://127.0.0.1:39999/loom/v1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "does not support browser login") {
		t.Fatalf("error = %v want browser login platform refusal, not token binding failure", err)
	}
}

func TestLogoutClearsSavedToken(t *testing.T) {
	isolateCmdConfigHome(t)
	server := "https://test.shengsuanyun.com/loom/v1"
	profile := saveTestProfile(t, server, platform.ShengSuanYun, "")
	t.Setenv(profile.TokenEnv, "")
	state := platform.LoadState()
	if _, err := state.SetToken(profile.Name, "sk-saved"); err != nil {
		t.Fatal(err)
	}
	if err := platform.SaveState(state); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"logout"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logout error = %v", err)
	}

	got, ok := platform.LoadState().FindProfile(server)
	if !ok || got.Token != "" {
		t.Fatalf("profile=%+v ok=%t want cleared token", got, ok)
	}
}

func TestLogoutReportsEnvironmentTokenWithoutRemovingIt(t *testing.T) {
	isolateCmdConfigHome(t)
	server := "https://loomloom.shengsuanyun.com/loom/v1"
	profile := saveTestProfile(t, server, platform.ShengSuanYun, "")
	state := platform.LoadState()
	if _, err := state.SetToken(profile.Name, "sk-browser"); err != nil {
		t.Fatal(err)
	}
	if err := platform.SaveState(state); err != nil {
		t.Fatal(err)
	}
	t.Setenv(profile.TokenEnv, "sk-environment")

	var output bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"logout", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logout error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatalf("decode logout JSON: %v\n%s", err, output.String())
	}
	if payload["token_removed"] != true {
		t.Fatalf("token_removed=%v want true", payload["token_removed"])
	}
	if payload["environment_token_set"] != true {
		t.Fatalf("environment_token_set=%v want true", payload["environment_token_set"])
	}
	got, ok := platform.LoadState().FindProfile(server)
	if !ok || got.Token != "" {
		t.Fatalf("profile=%+v ok=%t want cleared browser token", got, ok)
	}
	if value := strings.TrimSpace(os.Getenv(profile.TokenEnv)); value != "sk-environment" {
		t.Fatalf("environment token=%q want unchanged", value)
	}
}

func TestLogoutReportsNoEnvironmentToken(t *testing.T) {
	isolateCmdConfigHome(t)
	server := "https://loomloom.shengsuanyun.com/loom/v1"
	profile := saveTestProfile(t, server, platform.ShengSuanYun, "")
	t.Setenv(profile.TokenEnv, "")
	state := platform.LoadState()
	if _, err := state.SetToken(profile.Name, "sk-browser"); err != nil {
		t.Fatal(err)
	}
	if err := platform.SaveState(state); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"logout", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logout error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatalf("decode logout JSON: %v\n%s", err, output.String())
	}
	if payload["environment_token_set"] != false {
		t.Fatalf("environment_token_set=%v want false", payload["environment_token_set"])
	}
}

func TestLogoutTextDistinguishesBrowserAndEnvironmentCredentials(t *testing.T) {
	isolateCmdConfigHome(t)
	profile := saveTestProfile(t, "https://loomloom.shengsuanyun.com/loom/v1", platform.ShengSuanYun, "")
	t.Setenv(profile.TokenEnv, "sk-from-env")
	state := platform.LoadState()
	if _, err := state.SetToken(profile.Name, "sk-browser"); err != nil {
		t.Fatal(err)
	}
	if err := platform.SaveState(state); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"logout"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logout error = %v", err)
	}
	for _, want := range []string{
		"浏览器登录凭据已删除",
		"环境变量 API Token",
		"未被删除",
		"仍会优先使用",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("logout output missing %q:\n%s", want, output.String())
		}
	}
}

func TestLogoutWithoutSavedLoginReportsLocalCredentialState(t *testing.T) {
	isolateCmdConfigHome(t)
	profile := saveTestProfile(t, "https://loomloom.shengsuanyun.com/loom/v1", platform.ShengSuanYun, "")
	t.Setenv(profile.TokenEnv, "sk-from-env")

	var output bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"logout", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logout error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatalf("decode logout JSON: %v\n%s", err, output.String())
	}
	if payload["token_removed"] != false {
		t.Fatalf("token_removed=%v want false", payload["token_removed"])
	}
	if payload["environment_token_set"] != true {
		t.Fatalf("environment_token_set=%v want true", payload["environment_token_set"])
	}
}

func TestLogoutWithoutProfileIsIdempotent(t *testing.T) {
	isolateCmdConfigHome(t)

	var output bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"logout", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logout error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatalf("decode logout JSON: %v\n%s", err, output.String())
	}
	if payload["token_removed"] != false || payload["environment_token_set"] != false {
		t.Fatalf("payload=%v want no saved or environment credential", payload)
	}
}

func TestLogoutIgnoresRemovedBatchjobEnvironment(t *testing.T) {
	isolateCmdConfigHome(t)
	profile := saveTestProfile(t, "https://loomloom.shengsuanyun.com/loom/v1", platform.ShengSuanYun, "")
	t.Setenv(profile.TokenEnv, "")
	t.Setenv("BATCHJOB_TOKEN", "removed-token")

	var output bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"logout", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logout error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatalf("decode logout JSON: %v\n%s", err, output.String())
	}
	if payload["environment_token_set"] != false {
		t.Fatalf("environment_token_set=%v want false", payload["environment_token_set"])
	}
}

func TestLogoutClearsOnlyActiveProfileSavedToken(t *testing.T) {
	isolateCmdConfigHome(t)
	first := saveTestProfile(t, "https://first.example.com/loom/v1", platform.Custom, "")
	state := platform.LoadState()
	if _, err := state.SetToken(first.Name, "sk-first"); err != nil {
		t.Fatal(err)
	}
	if err := platform.SaveState(state); err != nil {
		t.Fatal(err)
	}

	second := saveTestProfile(t, "https://second.example.com/loom/v1", platform.Custom, "")
	state = platform.LoadState()
	if _, err := state.SetToken(second.Name, "sk-second"); err != nil {
		t.Fatal(err)
	}
	if err := platform.SaveState(state); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"logout"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logout error = %v", err)
	}

	loaded := platform.LoadState()
	gotFirst, ok := loaded.FindProfile(first.Name)
	if !ok || gotFirst.Token != "sk-first" {
		t.Fatalf("first profile=%+v ok=%t want preserved token", gotFirst, ok)
	}
	gotSecond, ok := loaded.FindProfile(second.Name)
	if !ok || gotSecond.Token != "" {
		t.Fatalf("second profile=%+v ok=%t want cleared token", gotSecond, ok)
	}
}

func TestConfiguredTokenFallsBackToSavedToken(t *testing.T) {
	isolateCmdConfigHome(t)
	server := "https://test.cogfoundry.ai/loom/v1"
	profile := saveTestProfile(t, server, platform.CogFoundry, "")
	state := platform.LoadState()
	if _, err := state.SetToken(profile.Name, "sk-saved"); err != nil {
		t.Fatal(err)
	}
	if err := platform.SaveState(state); err != nil {
		t.Fatal(err)
	}

	got, source, err := configuredToken(server)
	if err != nil {
		t.Fatal(err)
	}
	if got != "sk-saved" || source != "saved" {
		t.Fatalf("token=%q source=%q want saved token", got, source)
	}

	// The selected profile's environment variable still wins over the saved token.
	t.Setenv(profile.TokenEnv, "sk-from-env")
	got, source, err = configuredToken(server)
	if err != nil {
		t.Fatal(err)
	}
	if got != "sk-from-env" || source != profile.TokenEnv {
		t.Fatalf("token=%q source=%q want profile environment override", got, source)
	}
}

func TestVerifyLoginTokenUsesPrivateRunsProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loom/v1/marketListings":
			http.Error(w, `{"error":"market is unavailable for OPC tenants"}`, http.StatusForbidden)
		case "/loom/v1/users/me/runs":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := NewRootCmd()
	cmd.SetContext(context.Background())
	err := verifyLoginToken(cmd, &rootOptions{
		server:  server.URL + "/loom/v1",
		token:   "opc-token",
		timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("verifyLoginToken() error = %v", err)
	}
}
