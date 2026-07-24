package platform

import "testing"

func TestStateTokenPersistsAcrossSaveAndLoad(t *testing.T) {
	isolateConfigHome(t)
	if err := SaveState(State{
		Platform: CogFoundry,
		Server:   "https://test.shengsuanyun.com/loom/v1",
		Token:    "sk-saved",
	}); err != nil {
		t.Fatal(err)
	}

	loaded := LoadState()
	if loaded.Token != "sk-saved" {
		t.Fatalf("loaded token=%q want sk-saved", loaded.Token)
	}

	loaded.Token = ""
	if err := SaveState(loaded); err != nil {
		t.Fatal(err)
	}
	if got := LoadState(); got.Token != "" {
		t.Fatalf("token=%q want cleared", got.Token)
	}
}
