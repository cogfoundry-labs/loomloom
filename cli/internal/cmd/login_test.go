package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/platform"
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

func TestLogoutWithoutSavedLoginFails(t *testing.T) {
	isolateCmdConfigHome(t)
	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"logout"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no saved login") {
		t.Fatalf("error = %v want missing login", err)
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
