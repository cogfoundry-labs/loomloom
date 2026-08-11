package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/platform"
)

func saveServerProfiles(t *testing.T) (platform.Profile, platform.Profile) {
	t.Helper()
	var state platform.State
	first, err := state.UpsertVerified(
		"https://loomloom.shengsuanyun.com/loom/v1",
		platform.ShengSuanYun,
		"",
		time.Unix(1, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := state.UpsertVerified(
		"https://loomloom.cogfoundry.ai/loom/v1",
		platform.CogFoundry,
		"",
		time.Unix(2, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := platform.SaveState(state); err != nil {
		t.Fatal(err)
	}
	return first, second
}

func TestServerListDoesNotExposeTokenValue(t *testing.T) {
	isolateCmdConfigHome(t)
	first, _ := saveServerProfiles(t)
	t.Setenv(first.TokenEnv, "super-secret-token")
	opts := &rootOptions{output: "json"}
	cmd := newServerListCmd(opts)
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "super-secret-token") {
		t.Fatalf("output leaked token: %s", out.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload["servers"].([]any)) != 2 {
		t.Fatalf("payload=%v want two profiles", payload)
	}
}

func TestServerUseSwitchesActiveProfileAndCompatibilityMirror(t *testing.T) {
	isolateCmdConfigHome(t)
	first, _ := saveServerProfiles(t)
	opts := &rootOptions{output: "json"}
	cmd := newServerUseCmd(opts)
	cmd.SetArgs([]string{first.Name})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	state := platform.LoadState()
	if state.ActiveServer != first.Name || state.Server != first.Server || state.Platform != first.Platform {
		t.Fatalf("state=%+v want first profile active", state)
	}
}

func TestServerRemoveActiveDoesNotSelectAnotherProfile(t *testing.T) {
	isolateCmdConfigHome(t)
	t.Setenv("LOOMLOOM_SERVER", "https://legacy.shengsuanyun.com/loom/v1")
	_, second := saveServerProfiles(t)
	opts := &rootOptions{output: "json"}
	cmd := newServerRemoveCmd(opts)
	cmd.SetArgs([]string{second.Name})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	state := platform.LoadState()
	if state.ActiveServer != "" || state.Server != "" || len(state.Servers) != 1 {
		t.Fatalf("state=%+v want remaining profile without active selection", state)
	}
	if got := configuredServer(); got != "" {
		t.Fatalf("configuredServer=%q want removed active profile to stay unset", got)
	}
}
