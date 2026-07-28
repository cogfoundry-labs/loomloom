package cmd

import (
	"bytes"
	"encoding/json"
	"os"
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
	t.Setenv("LOOMLOOM_TOKEN", "")
	t.Setenv("BATCHJOB_TOKEN", "")
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

func TestLogoutReportsEnvironmentTokenWithoutRemovingIt(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		value string
	}{
		{name: "loomloom token", env: "LOOMLOOM_TOKEN", value: "sk-loomloom"},
		{name: "legacy batchjob token", env: "BATCHJOB_TOKEN", value: "sk-batchjob"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateCmdConfigHome(t)
			t.Setenv("LOOMLOOM_TOKEN", "")
			t.Setenv("BATCHJOB_TOKEN", "")
			t.Setenv(tt.env, tt.value)
			if err := platform.SaveState(platform.State{
				Platform: platform.ShengSuanYun,
				Server:   "https://loomloom.shengsuanyun.com/loom/v1",
				Token:    "sk-browser",
			}); err != nil {
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
			if payload["token_removed"] != true {
				t.Fatalf("token_removed=%v want true", payload["token_removed"])
			}
			if payload["environment_token_set"] != true {
				t.Fatalf("environment_token_set=%v want true", payload["environment_token_set"])
			}
			if got := platform.LoadState().Token; got != "" {
				t.Fatalf("saved browser token=%q want cleared", got)
			}
			if got := strings.TrimSpace(os.Getenv(tt.env)); got != tt.value {
				t.Fatalf("environment token=%q want unchanged", got)
			}
		})
	}
}

func TestLogoutReportsNoEnvironmentToken(t *testing.T) {
	isolateCmdConfigHome(t)
	t.Setenv("LOOMLOOM_TOKEN", "")
	t.Setenv("BATCHJOB_TOKEN", "")
	if err := platform.SaveState(platform.State{
		Platform: platform.ShengSuanYun,
		Server:   "https://loomloom.shengsuanyun.com/loom/v1",
		Token:    "sk-browser",
	}); err != nil {
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
	t.Setenv("LOOMLOOM_TOKEN", "sk-from-env")
	t.Setenv("BATCHJOB_TOKEN", "")
	if err := platform.SaveState(platform.State{
		Platform: platform.ShengSuanYun,
		Server:   "https://loomloom.shengsuanyun.com/loom/v1",
		Token:    "sk-browser",
	}); err != nil {
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
	t.Setenv("LOOMLOOM_TOKEN", "sk-from-env")
	t.Setenv("BATCHJOB_TOKEN", "")

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

func TestConfiguredTokenFallsBackToSavedToken(t *testing.T) {
	isolateCmdConfigHome(t)
	t.Setenv("LOOMLOOM_TOKEN", "")
	t.Setenv("BATCHJOB_TOKEN", "")
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
