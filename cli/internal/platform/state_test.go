package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func isolateConfigHome(t *testing.T) string {
	t.Helper()
	temp := t.TempDir()
	t.Setenv("HOME", temp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(temp, ".config"))
	return temp
}

func TestLoadStateMissingFileReturnsZero(t *testing.T) {
	isolateConfigHome(t)
	got := LoadState()
	if got.Platform != "" {
		t.Fatalf("LoadState platform=%q want empty", got.Platform)
	}
	if got.Server != "" {
		t.Fatalf("LoadState server=%q want empty", got.Server)
	}
}

func TestLoadStateDamagedJSONReturnsZero(t *testing.T) {
	isolateConfigHome(t)
	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath error=%v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll error=%v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatalf("WriteFile error=%v", err)
	}
	got := LoadState()
	if got.Platform != "" {
		t.Fatalf("LoadState platform=%q want empty", got.Platform)
	}
}

func TestSaveStateRoundTrip(t *testing.T) {
	isolateConfigHome(t)
	const server = "https://loomloom.shengsuanyun.com/loom/v1"
	if err := SaveState(State{Platform: ShengSuanYun, Server: server}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	got := LoadState()
	if got.Platform != ShengSuanYun {
		t.Fatalf("LoadState platform=%q want %q", got.Platform, ShengSuanYun)
	}
	if got.Server != server {
		t.Fatalf("LoadState server=%q want %q", got.Server, server)
	}
	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath error=%v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat error=%v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%#o want 0600", info.Mode().Perm())
	}
}

func TestSaveStateSkipsUnknownAndInvalidPlatform(t *testing.T) {
	isolateConfigHome(t)
	if err := SaveState(State{Platform: Unknown}); err != nil {
		t.Fatalf("SaveState unknown error=%v", err)
	}
	if err := SaveState(State{Platform: ID("invalid")}); err != nil {
		t.Fatalf("SaveState invalid error=%v", err)
	}
	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath error=%v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config file exists or unexpected error: %v", err)
	}
}

func TestSaveStateRepairsExistingFileMode(t *testing.T) {
	isolateConfigHome(t)
	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath error=%v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll error=%v", err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile error=%v", err)
	}
	if err := SaveState(State{Platform: ShengSuanYun}); err != nil {
		t.Fatalf("SaveState error=%v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat error=%v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%#o want 0600", info.Mode().Perm())
	}
}
