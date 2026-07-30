package platform

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateMigratesLegacySavedTokenIntoProfile(t *testing.T) {
	isolateConfigHome(t)
	path, err := StatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{
		"platform":"shengsuanyun",
		"server":"https://test.shengsuanyun.com/loom/v1",
		"token":"sk-saved"
	}`), 0600); err != nil {
		t.Fatal(err)
	}

	loaded := LoadState()
	profile, ok := loaded.ActiveProfile()
	if !ok || profile.Token != "sk-saved" {
		t.Fatalf("profile=%+v ok=%t want migrated saved token", profile, ok)
	}

	if _, err := loaded.SetToken(profile.Name, ""); err != nil {
		t.Fatal(err)
	}
	if err := SaveState(loaded); err != nil {
		t.Fatal(err)
	}
	profile, ok = LoadState().ActiveProfile()
	if !ok || profile.Token != "" {
		t.Fatalf("profile=%+v ok=%t want cleared token", profile, ok)
	}
}

func TestUpsertVerifiedPreservesSavedProfileToken(t *testing.T) {
	var state State
	profile, err := state.UpsertVerified(
		"https://test.shengsuanyun.com/loom/v1",
		ShengSuanYun,
		"",
		time.Unix(1, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.SetToken(profile.Name, "sk-saved"); err != nil {
		t.Fatal(err)
	}

	profile, err = state.UpsertVerified(
		profile.Server,
		profile.Platform,
		"",
		time.Unix(2, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Token != "sk-saved" {
		t.Fatalf("token=%q want preserved token", profile.Token)
	}
}
