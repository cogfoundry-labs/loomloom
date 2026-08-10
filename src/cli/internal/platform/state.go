package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Profile struct {
	Name       string `json:"name"`
	Platform   ID     `json:"platform"`
	Server     string `json:"server"`
	TokenEnv   string `json:"token_env"`
	Token      string `json:"token,omitempty"`
	VerifiedAt string `json:"verified_at,omitempty"`
}

type State struct {
	ActiveServer string    `json:"active_server,omitempty"`
	Servers      []Profile `json:"servers,omitempty"`
	Platform     ID        `json:"platform,omitempty"`
	Server       string    `json:"server,omitempty"`
	Token        string    `json:"token,omitempty"`
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
	return normalizeLoadedState(state)
}

func normalizeLoadedState(state State) State {
	legacyServer := strings.TrimSpace(state.Server)
	legacyToken := strings.TrimSpace(state.Token)
	state.Token = ""
	valid := make([]Profile, 0, len(state.Servers))
	seenNames := map[string]bool{}
	seenServers := map[string]bool{}
	for _, profile := range state.Servers {
		normalized, err := NormalizeServer(profile.Server)
		profile.Name = strings.TrimSpace(strings.ToLower(profile.Name))
		if err != nil || len(profile.Name) > 64 || !profileNamePattern.MatchString(profile.Name) {
			continue
		}
		profile.Server = normalized
		if seenNames[profile.Name] || seenServers[profile.Server] {
			continue
		}
		if inferred := InferFromServer(normalized); inferred.ID != Unknown {
			profile.Platform = inferred.ID
		}
		if profile.TokenEnv != "LOOMLOOM_TOKEN" &&
			profile.TokenEnv != TokenEnvName(profile.Name) {
			profile.TokenEnv = TokenEnvName(profile.Name)
		}
		profile.Token = strings.TrimSpace(profile.Token)
		seenNames[profile.Name] = true
		seenServers[profile.Server] = true
		valid = append(valid, profile)
	}
	state.Servers = valid

	if len(state.Servers) == 0 && strings.TrimSpace(state.Server) != "" {
		if normalized, err := NormalizeServer(state.Server); err == nil {
			p := InferFromServer(normalized)
			if stored, ok := ByID(state.Platform); ok && stored.ID != Unknown && p.ID == Custom {
				p = stored
			}
			name, nameErr := GenerateProfileName(normalized, p.ID, nil, "")
			if nameErr == nil {
				state.Servers = []Profile{{
					Name:     name,
					Platform: p.ID,
					Server:   normalized,
					TokenEnv: "LOOMLOOM_TOKEN",
					Token:    legacyToken,
				}}
				state.ActiveServer = name
			}
		}
	}
	if len(state.Servers) == 0 {
		if _, ok := ByID(state.Platform); !ok {
			state.Platform = ""
		}
		return state
	}
	if legacyToken != "" {
		profile, ok := state.FindProfile(legacyServer)
		if !ok {
			profile, ok = state.ActiveProfile()
		}
		if ok {
			for i := range state.Servers {
				if state.Servers[i].Name == profile.Name && state.Servers[i].Token == "" {
					state.Servers[i].Token = legacyToken
					break
				}
			}
		}
	}
	state.syncCompatibility()
	return state
}

func (s State) ActiveProfile() (Profile, bool) {
	for _, profile := range s.Servers {
		if profile.Name == s.ActiveServer {
			return profile, true
		}
	}
	return Profile{}, false
}

func (s State) FindProfile(value string) (Profile, bool) {
	value = strings.TrimSpace(value)
	for _, profile := range s.Servers {
		if profile.Name == value {
			return profile, true
		}
	}
	if normalized, err := NormalizeServer(value); err == nil {
		for _, profile := range s.Servers {
			if profile.Server == normalized {
				return profile, true
			}
		}
	}
	return Profile{}, false
}

func (s *State) UpsertVerified(server string, platformID ID, requestedName string, verifiedAt time.Time) (Profile, error) {
	normalized, err := NormalizeServer(server)
	if err != nil {
		return Profile{}, err
	}
	name, err := GenerateProfileName(normalized, platformID, s.Servers, requestedName)
	if err != nil {
		return Profile{}, err
	}
	profile := Profile{
		Name:       name,
		Platform:   platformID,
		Server:     normalized,
		TokenEnv:   TokenEnvName(name),
		VerifiedAt: verifiedAt.UTC().Format(time.RFC3339),
	}
	for i, existing := range s.Servers {
		if existing.Server == normalized {
			profile.Token = existing.Token
			s.Servers[i] = profile
			s.ActiveServer = profile.Name
			s.syncCompatibility()
			return profile, nil
		}
	}
	s.Servers = append(s.Servers, profile)
	s.ActiveServer = profile.Name
	s.syncCompatibility()
	return profile, nil
}

func (s *State) SetToken(value, token string) (Profile, error) {
	profile, ok := s.FindProfile(value)
	if !ok {
		return Profile{}, fmt.Errorf("server profile %q not found; run `loomloom server list`", value)
	}
	for i := range s.Servers {
		if s.Servers[i].Name == profile.Name {
			s.Servers[i].Token = strings.TrimSpace(token)
			profile = s.Servers[i]
			return profile, nil
		}
	}
	return Profile{}, fmt.Errorf("server profile %q not found; run `loomloom server list`", value)
}

func (s *State) Use(value string) (Profile, error) {
	profile, ok := s.FindProfile(value)
	if !ok {
		return Profile{}, fmt.Errorf("server profile %q not found; run `loomloom server list`", value)
	}
	s.ActiveServer = profile.Name
	s.syncCompatibility()
	return profile, nil
}

func (s *State) Remove(value string) (Profile, error) {
	profile, ok := s.FindProfile(value)
	if !ok {
		return Profile{}, fmt.Errorf("server profile %q not found; run `loomloom server list`", value)
	}
	remaining := make([]Profile, 0, len(s.Servers)-1)
	for _, item := range s.Servers {
		if item.Server != profile.Server {
			remaining = append(remaining, item)
		}
	}
	s.Servers = remaining
	if s.ActiveServer == profile.Name {
		s.ActiveServer = ""
		s.Platform = ""
		s.Server = ""
	} else {
		s.syncCompatibility()
	}
	return profile, nil
}

func (s *State) syncCompatibility() {
	active, ok := s.ActiveProfile()
	if !ok {
		s.Platform = ""
		s.Server = ""
		return
	}
	s.Platform = active.Platform
	s.Server = active.Server
}

func SaveState(state State) error {
	state = normalizeLoadedState(state)
	path, err := StatePath()
	if err != nil {
		return err
	}
	if len(state.Servers) == 0 && state.Server == "" && state.Platform == "" {
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			return nil
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	temp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}
