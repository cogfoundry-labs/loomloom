package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type State struct {
	Platform ID `json:"platform,omitempty"`
}

func StatePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "loomloom", "config.json"), nil
}

func LoadState() State {
	path, err := StatePath()
	if err != nil {
		return State{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}
	}
	if _, ok := ByID(state.Platform); !ok {
		return State{}
	}
	return state
}

func SaveState(state State) error {
	if state.Platform == "" || state.Platform == Unknown {
		return nil
	}
	if _, ok := ByID(state.Platform); !ok {
		return nil
	}
	path, err := StatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}
