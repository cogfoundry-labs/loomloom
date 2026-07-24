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

func TestLogoutClearsSavedToken(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{
		Platform: platform.CogFoundry,
		Server:   "https://test.shengsuanyun.com/loom/v1",
		Token:    "sk-saved",
	}); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"logout"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logout error = %v", err)
	}

	if got := platform.LoadState(); got.Token != "" {
		t.Fatalf("token=%q want cleared", got.Token)
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

func TestConfiguredTokenFallsBackToSavedToken(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{
		Platform: platform.CogFoundry,
		Server:   "https://test.cogfoundry.ai/loom/v1",
		Token:    "sk-saved",
	}); err != nil {
		t.Fatal(err)
	}

	if got := configuredToken(); got != "sk-saved" {
		t.Fatalf("token=%q want saved token", got)
	}

	// Environment variables still win over the saved token.
	t.Setenv("LOOMLOOM_TOKEN", "sk-from-env")
	if got := configuredToken(); got != "sk-from-env" {
		t.Fatalf("token=%q want env override", got)
	}
}
