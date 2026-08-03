package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cogfoundry-labs/loomloom/cli/internal/platform"
)

func resolvePlatform(opts *rootOptions) (platform.Platform, error) {
	inferred := platform.InferFromServer(opts.server)
	if inferred.ID != platform.Unknown {
		state := platform.LoadState()
		if inferred.ID == platform.Custom && len(state.Servers) == 0 && strings.TrimSpace(state.Server) == "" {
			if stored, found := platform.ByID(state.Platform); found &&
				stored.ID != platform.Unknown && stored.ID != platform.Custom {
				return stored, nil
			}
		}
		if err := validatePlatformHint(inferred); err != nil {
			return platform.Platform{}, err
		}
		return inferred, nil
	}
	state := platform.LoadState()
	if active, ok := state.ActiveProfile(); ok {
		if stored, found := platform.ByID(active.Platform); found {
			return stored, nil
		}
	}
	return platform.UnknownPlatform(), nil
}

func validatePlatformHint(inferred platform.Platform) error {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv("LOOMLOOM_PLATFORM")))
	if raw == "" {
		return nil
	}
	hinted, ok := platform.ByID(platform.ID(raw))
	if !ok || hinted.ID == platform.Unknown {
		return fmt.Errorf("unsupported LOOMLOOM_PLATFORM %q; use shengsuanyun, cogfoundry, or custom", raw)
	}
	if inferred.ID != platform.Unknown && hinted.ID != inferred.ID {
		return fmt.Errorf("LOOMLOOM_PLATFORM=%s conflicts with the platform inferred from LOOMLOOM_SERVER (%s)", hinted.ID, inferred.ID)
	}
	return nil
}

func validateTokenPlatform(opts *rootOptions, allowUnverified bool) error {
	resolved, err := resolvePlatform(opts)
	if err != nil {
		return err
	}
	if allowUnverified || !opts.enforceServerVerification {
		return nil
	}
	if strings.TrimSpace(opts.server) != "" {
		state := platform.LoadState()
		if _, ok := state.FindProfile(opts.server); !ok {
			return fmt.Errorf("server has not passed LoomLoom Doctor; run `loomloom doctor --server %q --output json` first", opts.server)
		}
	}
	if resolved.ID == platform.Unknown {
		return nil
	}
	if !resolved.Operational {
		return fmt.Errorf("platform %s is not operational", resolved.DisplayName)
	}
	return nil
}

func credentialMessageForPlatform(p platform.Platform) (string, string) {
	switch p.ID {
	case platform.ShengSuanYun:
		return "missing_token", missingShengSuanYunTokenMessage
	case platform.CogFoundry:
		return "missing_token", missingCogFoundryTokenMessage
	case platform.Custom:
		return "missing_token", missingCustomTokenMessage
	default:
		return "choose_platform", choosePlatformMessage
	}
}

func tokenAuthenticationFailureMessage() string {
	return tokenAuthenticationFailedMessage
}

func insufficientBalanceMessage(opts *rootOptions) string {
	p, err := resolvePlatform(opts)
	if err == nil && p.ID == platform.ShengSuanYun {
		return insufficientShengSuanYunBalanceMessage
	}
	if err == nil && p.ID == platform.CogFoundry {
		return insufficientCogFoundryBalanceMessage
	}
	return ""
}

func maybeInsufficientBalanceError(opts *rootOptions, balance *templateBalanceCheck) error {
	if balance == nil || balance.IsSufficient {
		return nil
	}
	if message := insufficientBalanceMessage(opts); message != "" {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func maybeMapInsufficientBalanceError(opts *rootOptions, values map[string]any) error {
	if values == nil {
		return nil
	}
	balance, ok := mapBalanceCheck(values)
	if !ok || balance.isSufficient {
		return nil
	}
	if message := insufficientBalanceMessage(opts); message != "" {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func mapInsufficientBalance(values map[string]any) bool {
	balance, ok := mapBalanceCheck(values)
	return ok && !balance.isSufficient
}

type decodedBalanceCheck struct {
	isSufficient bool
}

func mapBalanceCheck(values map[string]any) (decodedBalanceCheck, bool) {
	raw, ok := values["balanceCheck"]
	if !ok {
		raw, ok = values["balance_check"]
	}
	if !ok || raw == nil {
		return decodedBalanceCheck{}, false
	}
	balance, ok := raw.(map[string]any)
	if !ok {
		return decodedBalanceCheck{}, false
	}
	isSufficient, ok := balance["isSufficient"].(bool)
	if !ok {
		isSufficient, ok = balance["is_sufficient"].(bool)
	}
	if !ok {
		return decodedBalanceCheck{}, false
	}
	return decodedBalanceCheck{isSufficient: isSufficient}, true
}
