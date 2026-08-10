package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func isolateConfigHome(t *testing.T) string {
	t.Helper()
	temp := t.TempDir()
	t.Setenv("HOME", temp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(temp, ".config"))
	return temp
}

func TestLoadStateMissingOrDamagedFileReturnsZero(t *testing.T) {
	isolateConfigHome(t)
	if got := LoadState(); len(got.Servers) != 0 {
		t.Fatalf("missing state=%+v want zero", got)
	}
	path, _ := StatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(); len(got.Servers) != 0 {
		t.Fatalf("damaged state=%+v want zero", got)
	}
}

func TestLoadStateMigratesLegacyServerInMemory(t *testing.T) {
	isolateConfigHome(t)
	path, _ := StatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"platform":"shengsuanyun","server":"https://loomloom.shengsuanyun.com/loom/v1"}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	state := LoadState()
	active, ok := state.ActiveProfile()
	if !ok {
		t.Fatalf("state=%+v want migrated active profile", state)
	}
	if active.Name != "shengsuanyun" || active.TokenEnv != "LOOMLOOM_TOKEN" {
		t.Fatalf("active=%+v want legacy token mapping", active)
	}
}

func TestStateUpsertUseRemoveRoundTrip(t *testing.T) {
	isolateConfigHome(t)
	var state State
	first, err := state.UpsertVerified(
		"https://loomloom.shengsuanyun.com/loom/v1",
		ShengSuanYun,
		"",
		time.Unix(1, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := state.UpsertVerified(
		"https://loomloom-integration.test.cogfoundry.ai/loom/v1",
		CogFoundry,
		"",
		time.Unix(2, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveServer != second.Name || len(state.Servers) != 2 {
		t.Fatalf("state=%+v want second active", state)
	}
	if _, err := state.Use(first.Name); err != nil {
		t.Fatal(err)
	}
	if state.Server != first.Server || state.Platform != first.Platform {
		t.Fatalf("compatibility mirror not updated: %+v", state)
	}
	if err := SaveState(state); err != nil {
		t.Fatal(err)
	}
	loaded := LoadState()
	if len(loaded.Servers) != 2 || loaded.ActiveServer != first.Name {
		t.Fatalf("loaded=%+v want round trip", loaded)
	}
	if _, err := loaded.Remove(first.Name); err != nil {
		t.Fatal(err)
	}
	if loaded.ActiveServer != "" || loaded.Server != "" {
		t.Fatalf("remove active state=%+v want no implicit fallback", loaded)
	}
}

func TestSaveStateUsesPrivateFileModeAndCanWriteEmptyState(t *testing.T) {
	isolateConfigHome(t)
	var state State
	if _, err := state.UpsertVerified("https://api.example.com/loom/v1", Custom, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(state); err != nil {
		t.Fatal(err)
	}
	path, _ := StatePath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%#o want 0600", info.Mode().Perm())
	}
	if err := SaveState(State{}); err != nil {
		t.Fatal(err)
	}
	loaded := LoadState()
	if len(loaded.Servers) != 0 {
		t.Fatalf("loaded=%+v want empty state", loaded)
	}
}

func TestLoadStateRepairsUnexpectedTokenEnvironmentName(t *testing.T) {
	isolateConfigHome(t)
	path, _ := StatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{
		"active_server":"company",
		"servers":[{
			"name":"company",
			"platform":"custom",
			"server":"https://api.company.com/loom/v1",
			"token_env":"BATCHJOB_TOKEN"
		}]
	}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	active, ok := LoadState().ActiveProfile()
	if !ok || active.TokenEnv != "LOOMLOOM_TOKEN_COMPANY" {
		t.Fatalf("active=%+v ok=%t want repaired token environment", active, ok)
	}
}
