package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/platform"
	"github.com/spf13/cobra"
)

func TestMain(m *testing.M) {
	temp, err := os.MkdirTemp("", "loomloom-cmd-tests-")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("XDG_CONFIG_HOME", filepath.Join(temp, ".config"))
	for _, key := range []string{
		"LOOMLOOM_SERVER",
		"LOOMLOOM_TOKEN",
		"LOOMLOOM_PLATFORM",
	} {
		_ = os.Unsetenv(key)
	}
	code := m.Run()
	_ = os.RemoveAll(temp)
	os.Exit(code)
}

func isolateCmdConfigHome(t *testing.T) {
	t.Helper()
	temp := t.TempDir()
	t.Setenv("HOME", temp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(temp, ".config"))
}

func saveTestProfile(t *testing.T, server string, platformID platform.ID, tokenEnv string) platform.Profile {
	t.Helper()
	state := platform.LoadState()
	profile, err := state.UpsertVerified(server, platformID, "", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if tokenEnv != "" {
		for i := range state.Servers {
			if state.Servers[i].Name == profile.Name {
				state.Servers[i].TokenEnv = tokenEnv
				profile.TokenEnv = tokenEnv
			}
		}
	}
	if err := platform.SaveState(state); err != nil {
		t.Fatal(err)
	}
	return profile
}

func newRootCmdWithVerifiedServer(t *testing.T, server string) *cobra.Command {
	t.Helper()
	isolateCmdConfigHome(t)
	saveTestProfile(t, server, platform.InferFromServer(server).ID, "")
	return NewRootCmd()
}

func TestRootRejectsUnsupportedOutputFormat(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--output", "yaml", "doctor"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `unsupported output format "yaml"`) {
		t.Fatalf("error=%v want unsupported output format", err)
	}
}

func TestRootReadsVerboseEnvironment(t *testing.T) {
	t.Setenv("LOOMLOOM_VERBOSE", "true")
	cmd := NewRootCmd()
	flag := cmd.PersistentFlags().Lookup("verbose")
	if flag == nil || flag.DefValue != "true" {
		t.Fatalf("verbose default=%v want true from environment", flag)
	}
}

func TestRootWritesStableUpdateNoticeOnlyForTextOutput(t *testing.T) {
	originalCheck := checkStableUpdateNotice
	t.Cleanup(func() { checkStableUpdateNotice = originalCheck })
	checkStableUpdateNotice = func(context.Context) (string, error) {
		return "new LoomLoom stable release available: v1.2.4", nil
	}

	cmd := &cobra.Command{Use: "template"}
	var textErr bytes.Buffer
	cmd.SetErr(&textErr)
	writeStableUpdateNotice(cmd, &rootOptions{output: "text"})
	if !strings.Contains(textErr.String(), "new LoomLoom stable release available: v1.2.4") {
		t.Fatalf("text notice=%q", textErr.String())
	}

	var jsonErr bytes.Buffer
	cmd.SetErr(&jsonErr)
	writeStableUpdateNotice(cmd, &rootOptions{output: "json"})
	if jsonErr.Len() != 0 {
		t.Fatalf("JSON output received update notice: %q", jsonErr.String())
	}
}

func TestRootLetsDoctorReturnStructuredInvalidServerResult(t *testing.T) {
	isolateCmdConfigHome(t)
	cmd := NewRootCmd()
	cmd.SetArgs([]string{
		"doctor",
		"--server", "http://api.example.com/loom/v1",
		"--output", "json",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor error=%v output=%s", err, out.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output=%q error=%v", out.String(), err)
	}
	if payload["healthy"] != false || payload["next_action"] != "fix_server" || payload["message"] != "invalid server URL" {
		t.Fatalf("payload=%v want structured invalid server result", payload)
	}
}

func TestRootBlocksEnvironmentOnlyServerBeforeBusinessRequest(t *testing.T) {
	isolateCmdConfigHome(t)
	const server = "https://unverified.example.com/loom/v1"
	t.Setenv("LOOMLOOM_SERVER", server)
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"template", "list", "--output", "json"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "has not passed LoomLoom Doctor") {
		t.Fatalf("error=%v want Doctor requirement", err)
	}
	if state := platform.LoadState(); len(state.Servers) != 0 {
		t.Fatalf("environment-only server was registered before Doctor: %+v", state)
	}
}

func TestRootBlocksExplicitTokenlessServerBeforeBusinessRequest(t *testing.T) {
	isolateCmdConfigHome(t)
	cmd := NewRootCmd()
	cmd.SetArgs([]string{
		"--server", "https://unverified.example.com/loom/v1",
		"template", "list",
		"--output", "json",
	})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "has not passed LoomLoom Doctor") {
		t.Fatalf("error=%v want Doctor requirement", err)
	}
}

func TestConfiguredServerPrefersActiveProfileOverLegacyEnvironment(t *testing.T) {
	isolateCmdConfigHome(t)
	const activeServer = "https://active.cogfoundry.ai/loom/v1"
	saveTestProfile(t, activeServer, platform.CogFoundry, "")
	t.Setenv("LOOMLOOM_SERVER", "https://legacy.shengsuanyun.com/loom/v1")
	if got := configuredServer(); got != activeServer {
		t.Fatalf("configuredServer=%q want active profile %q", got, activeServer)
	}
}

func TestConfiguredServerUsesEnvironmentWithoutRegisteringProfile(t *testing.T) {
	isolateCmdConfigHome(t)
	const server = "https://new.shengsuanyun.com/loom/v1"
	t.Setenv("LOOMLOOM_SERVER", server)
	if got := configuredServer(); got != server {
		t.Fatalf("configuredServer=%q want environment server", got)
	}
	if state := platform.LoadState(); len(state.Servers) != 0 {
		t.Fatalf("environment-only server was registered before Doctor: %+v", state)
	}
}

func TestConfiguredServerIgnoresRemovedBatchjobEnvironment(t *testing.T) {
	isolateCmdConfigHome(t)
	t.Setenv("BATCHJOB_SERVER", "https://removed.example.com/loom/v1")
	if got := configuredServer(); got != "" {
		t.Fatalf("configuredServer=%q want removed BATCHJOB_SERVER ignored", got)
	}
}

func TestConfiguredTokenUsesActiveProfileTokenEnvironment(t *testing.T) {
	isolateCmdConfigHome(t)
	const server = "https://loomloom.cogfoundry.ai/loom/v1"
	profile := saveTestProfile(t, server, platform.CogFoundry, "")
	t.Setenv(profile.TokenEnv, "profile-token")
	t.Setenv("LOOMLOOM_TOKEN", "legacy-token")
	t.Setenv("LOOMLOOM_SERVER", "https://legacy.shengsuanyun.com/loom/v1")
	token, source, err := configuredToken(server)
	if err != nil {
		t.Fatal(err)
	}
	if token != "profile-token" || source != profile.TokenEnv {
		t.Fatalf("token=%q source=%q want profile token from %q", token, source, profile.TokenEnv)
	}
}

func TestConfiguredTokenIgnoresRemovedBatchjobEnvironment(t *testing.T) {
	isolateCmdConfigHome(t)
	t.Setenv("BATCHJOB_TOKEN", "removed-token")
	token, source, err := configuredToken("https://api.example.com/loom/v1")
	if err != nil {
		t.Fatal(err)
	}
	if token != "" || source != "" {
		t.Fatalf("token=%q source=%q want removed BATCHJOB_TOKEN ignored", token, source)
	}
}

func TestDoctorRejectsConflictingEnvironmentPairBeforeRequest(t *testing.T) {
	isolateCmdConfigHome(t)
	var requests int
	activeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "must not be called", http.StatusInternalServerError)
	}))
	defer activeServer.Close()

	saveTestProfile(t, activeServer.URL+"/loom/v1", platform.Custom, "LOOMLOOM_TOKEN")
	t.Setenv("LOOMLOOM_SERVER", "https://loomloom-integration.test.cogfoundry.ai/loom/v1")
	t.Setenv("LOOMLOOM_TOKEN", "new-server-token")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"doctor", "--output", "json"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "LOOMLOOM_SERVER conflicts with the selected Server") {
		t.Fatalf("error=%v want conflicting Server/Token pair rejection", err)
	}
	if requests != 0 {
		t.Fatalf("requests=%d want no request before credential pair validation", requests)
	}
}

func TestDoctorExplicitServerAndTokenBypassConflictingLegacyEnvironment(t *testing.T) {
	isolateCmdConfigHome(t)
	activeServer := healthyDoctorServer(t, http.StatusOK)
	defer activeServer.Close()
	targetServer := healthyDoctorServer(t, http.StatusOK)
	defer targetServer.Close()

	saveTestProfile(t, activeServer.URL+"/loom/v1", platform.Custom, "LOOMLOOM_TOKEN")
	t.Setenv("LOOMLOOM_SERVER", activeServer.URL+"/loom/v1")
	t.Setenv("LOOMLOOM_TOKEN", "active-token")
	t.Setenv("LOOMLOOM_CLI_RELEASE_API", targetServer.URL+"/release")

	cmd := NewRootCmd()
	cmd.SetArgs([]string{
		"doctor",
		"--server", targetServer.URL + "/loom/v1",
		"--token", "target-token",
		"--output", "json",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor error=%v output=%s", err, out.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode output=%q error=%v", out.String(), err)
	}
	if payload["healthy"] != true || payload["server"] != targetServer.URL+"/loom/v1" {
		t.Fatalf("payload=%v want explicit target Server healthy", payload)
	}
}

func TestValidateTokenPlatformAllowsCogFoundryAndCustom(t *testing.T) {
	isolateCmdConfigHome(t)
	for _, server := range []string{
		"https://api.cogfoundry.ai/loom/v1",
		"https://api.example.com/loom/v1",
	} {
		if err := validateTokenPlatform(&rootOptions{server: server}, true); err != nil {
			t.Fatalf("server=%s error=%v want allowed for Doctor", server, err)
		}
	}
}

func TestValidateTokenPlatformBlocksEveryUnverifiedBusinessServer(t *testing.T) {
	for _, token := range []string{"", "token-1"} {
		t.Run("token="+token, func(t *testing.T) {
			isolateCmdConfigHome(t)
			opts := &rootOptions{
				server:                    "https://api.example.com/loom/v1",
				token:                     token,
				enforceServerVerification: true,
			}
			err := validateTokenPlatform(opts, false)
			if err == nil || !strings.Contains(err.Error(), "has not passed LoomLoom Doctor") {
				t.Fatalf("token=%q error=%v want Doctor requirement", token, err)
			}
		})
	}
}

func TestValidateTokenPlatformAllowsMigratedLegacyConfig(t *testing.T) {
	isolateCmdConfigHome(t)
	const server = "https://loomloom.shengsuanyun.com/loom/v1"
	path, err := platform.StatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"platform":"shengsuanyun","server":"`+server+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateTokenPlatform(&rootOptions{
		server:                    server,
		enforceServerVerification: true,
	}, false); err != nil {
		t.Fatalf("legacy verified config error=%v want allowed", err)
	}
}

func TestResolvePlatformRejectsConflictingEnvironmentHint(t *testing.T) {
	isolateCmdConfigHome(t)
	t.Setenv("LOOMLOOM_PLATFORM", "shengsuanyun")
	_, err := resolvePlatform(&rootOptions{server: "https://api.cogfoundry.ai/loom/v1"})
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("error=%v want platform conflict", err)
	}
}

func TestInsufficientBalanceMessagesArePlatformSpecific(t *testing.T) {
	tests := []struct {
		server  string
		want    string
		wantURL string
	}{
		{"https://api.shengsuanyun.com/loom/v1", insufficientShengSuanYunBalanceMessage, "https://console.shengsuanyun.com/user/recharge"},
		{"https://api.cogfoundry.ai/loom/v1", insufficientCogFoundryBalanceMessage, "https://console.cogfoundry.ai/credits"},
	}
	for _, tt := range tests {
		err := maybeInsufficientBalanceError(
			&rootOptions{server: tt.server},
			&templateBalanceCheck{IsSufficient: false},
		)
		if err == nil || err.Error() != tt.want {
			t.Fatalf("server=%s error=%v want %q", tt.server, err, tt.want)
		}
		if !strings.Contains(err.Error(), tt.wantURL) {
			t.Fatalf("server=%s error=%v want URL %q", tt.server, err, tt.wantURL)
		}
	}
}
