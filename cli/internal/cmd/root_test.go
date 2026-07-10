package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/client"
	"github.com/Cogfoundry-ai/loomloom/cli/internal/platform"
)

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

func isolateCmdConfigHome(t *testing.T) {
	t.Helper()
	temp := t.TempDir()
	t.Setenv("HOME", temp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(temp, ".config"))
}

func TestRootReadsServerFromStoredConfig(t *testing.T) {
	isolateCmdConfigHome(t)
	const server = "https://loomloom.shengsuanyun.com/loom/v1"
	if err := platform.SaveState(platform.State{Platform: platform.ShengSuanYun, Server: server}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	cmd := NewRootCmd()
	flag := cmd.PersistentFlags().Lookup("server")
	if flag == nil || flag.Value.String() != server {
		t.Fatalf("server default=%v want %q from stored config", flag, server)
	}
}

func TestRootEnvironmentServerOverridesStoredConfig(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{
		Platform: platform.ShengSuanYun,
		Server:   "https://stored.shengsuanyun.com/loom/v1",
	}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	const envServer = "https://env.shengsuanyun.com/loom/v1"
	t.Setenv("LOOMLOOM_SERVER", envServer)
	cmd := NewRootCmd()
	flag := cmd.PersistentFlags().Lookup("server")
	if flag == nil || flag.Value.String() != envServer {
		t.Fatalf("server default=%v want %q from environment", flag, envServer)
	}
}

func TestRootLegacyEnvironmentServerOverridesStoredConfig(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{
		Platform: platform.ShengSuanYun,
		Server:   "https://stored.shengsuanyun.com/loom/v1",
	}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	const envServer = "https://batchjob.shengsuanyun.com/loom/v1"
	t.Setenv("BATCHJOB_SERVER", envServer)
	cmd := NewRootCmd()
	flag := cmd.PersistentFlags().Lookup("server")
	if flag == nil || flag.Value.String() != envServer {
		t.Fatalf("server default=%v want %q from legacy environment", flag, envServer)
	}
}

func TestRootBlankEnvironmentServerFallsBackToStoredConfig(t *testing.T) {
	isolateCmdConfigHome(t)
	const storedServer = "https://stored.shengsuanyun.com/loom/v1"
	if err := platform.SaveState(platform.State{
		Platform: platform.ShengSuanYun,
		Server:   storedServer,
	}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	t.Setenv("LOOMLOOM_SERVER", "  ")
	cmd := NewRootCmd()
	flag := cmd.PersistentFlags().Lookup("server")
	if flag == nil || flag.Value.String() != storedServer {
		t.Fatalf("server default=%v want %q from stored config", flag, storedServer)
	}
}

func TestValidateTokenPlatformBlocksCogFoundry(t *testing.T) {
	isolateCmdConfigHome(t)
	opts := &rootOptions{
		server: "https://api.cogfoundry.ai/loom/v1",
		token:  "token-1",
	}
	err := validateTokenPlatform(opts)
	if err == nil || !strings.Contains(err.Error(), cogFoundryUnavailableMessage) {
		t.Fatalf("error=%v want CogFoundry unavailable message", err)
	}
}

func TestValidateTokenPlatformBlocksBoundPlatformConflict(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{Platform: platform.ShengSuanYun}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	opts := &rootOptions{
		server: "https://api.cogfoundry.ai/loom/v1",
		token:  "token-1",
	}
	err := validateTokenPlatform(opts)
	if err == nil || !strings.Contains(err.Error(), cogFoundryUnavailableMessage) {
		t.Fatalf("error=%v want CogFoundry unavailable message", err)
	}
}

func TestResolvePlatformUsesStoredPlatformForUnknownHost(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{Platform: platform.ShengSuanYun}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	got, err := resolvePlatform(&rootOptions{server: "http://127.0.0.1:8080/loom/v1"})
	if err != nil {
		t.Fatalf("resolvePlatform error=%v", err)
	}
	if got.ID != platform.ShengSuanYun {
		t.Fatalf("platform=%q want %q", got.ID, platform.ShengSuanYun)
	}
}

func TestResolvePlatformRejectsInvalidEnv(t *testing.T) {
	isolateCmdConfigHome(t)
	t.Setenv("LOOMLOOM_PLATFORM", "typo")
	_, err := resolvePlatform(&rootOptions{server: "https://loomloom.shengsuanyun.com/loom/v1"})
	if err == nil || !strings.Contains(err.Error(), "unsupported LOOMLOOM_PLATFORM") {
		t.Fatalf("error=%v want unsupported LOOMLOOM_PLATFORM", err)
	}
}

func TestMaybePersistVerifiedPlatformRequiresVerification(t *testing.T) {
	isolateCmdConfigHome(t)
	opts := &rootOptions{server: "https://loomloom.shengsuanyun.com/loom/v1"}
	maybePersistVerifiedPlatform(opts, false)
	if got := platform.LoadState(); got.Platform != "" {
		t.Fatalf("platform=%q want empty without verification", got.Platform)
	}
	maybePersistVerifiedPlatform(opts, true)
	got := platform.LoadState()
	if got.Platform != platform.ShengSuanYun {
		t.Fatalf("platform=%q want %q", got.Platform, platform.ShengSuanYun)
	}
	if got.Server != opts.server {
		t.Fatalf("server=%q want %q", got.Server, opts.server)
	}
}

func TestMaybePersistVerifiedPlatformSkipsRewriteWhenAlreadyBoundWithSameServer(t *testing.T) {
	isolateCmdConfigHome(t)
	path, err := platform.StatePath()
	if err != nil {
		t.Fatalf("StatePath error=%v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll error=%v", err)
	}
	original := []byte("{\n  \"platform\": \"shengsuanyun\",\n  \"server\": \"https://loomloom.shengsuanyun.com/loom/v1\",\n  \"kept\": true\n}\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("WriteFile error=%v", err)
	}

	opts := &rootOptions{server: "https://loomloom.shengsuanyun.com/loom/v1"}
	maybePersistVerifiedPlatform(opts, true)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error=%v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("config was rewritten:\n%s", string(got))
	}
}

func TestAuthenticatedProductPath(t *testing.T) {
	tests := []struct {
		name string
		meta client.SuccessMeta
		want bool
	}{
		{name: "users me", meta: client.SuccessMeta{Path: "/loom/v1/users/me/executables", Authed: true}, want: true},
		{name: "creators me", meta: client.SuccessMeta{Path: "/loom/v1/creators/me/earnings", Authed: true}, want: true},
		{name: "public authed", meta: client.SuccessMeta{Path: "/loom/v1/marketListings", Authed: true}, want: false},
		{name: "users unauthenticated", meta: client.SuccessMeta{Path: "/loom/v1/users/me/executables"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAuthenticatedProductPath(tt.meta); got != tt.want {
				t.Fatalf("isAuthenticatedProductPath=%t want %t", got, tt.want)
			}
		})
	}
}

func TestMaybeInsufficientBalanceErrorUsesBoundShengSuanYunMessage(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{Platform: platform.ShengSuanYun}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	opts := &rootOptions{server: "http://127.0.0.1:8080/loom/v1"}
	err := maybeInsufficientBalanceError(opts, &templateBalanceCheck{IsSufficient: false})
	if err == nil || err.Error() != insufficientShengSuanYunBalanceMessage {
		t.Fatalf("error=%v want fixed ShengSuanYun balance message", err)
	}
}

func TestMaybeMapInsufficientBalanceErrorUsesBoundShengSuanYunMessage(t *testing.T) {
	isolateCmdConfigHome(t)
	if err := platform.SaveState(platform.State{Platform: platform.ShengSuanYun}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	opts := &rootOptions{server: "http://127.0.0.1:8080/loom/v1"}
	err := maybeMapInsufficientBalanceError(opts, map[string]any{
		"balanceCheck": map[string]any{
			"isSufficient": false,
		},
	})
	if err == nil || err.Error() != insufficientShengSuanYunBalanceMessage {
		t.Fatalf("error=%v want fixed ShengSuanYun balance message", err)
	}
}
