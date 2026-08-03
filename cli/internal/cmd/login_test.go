package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/cogfoundry-labs/loomloom/cli/internal/platform"
)

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
