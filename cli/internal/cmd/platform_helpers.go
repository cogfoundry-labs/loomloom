package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/client"
	"github.com/Cogfoundry-ai/loomloom/cli/internal/platform"
)

func resolvePlatform(opts *rootOptions) (platform.Platform, error) {
	if envPlatform, ok, err := platformFromEnv(); ok || err != nil {
		return envPlatform, err
	}
	inferred := platform.InferFromServer(opts.server)
	if inferred.ID != platform.Unknown {
		return inferred, nil
	}
	state := platform.LoadState()
	if stored, ok := platform.ByID(state.Platform); ok && stored.ID != platform.Unknown {
		return stored, nil
	}
	return platform.UnknownPlatform(), nil
}

func platformFromEnv() (platform.Platform, bool, error) {
	raw := strings.TrimSpace(os.Getenv("LOOMLOOM_PLATFORM"))
	if raw == "" {
		return platform.Platform{}, false, nil
	}
	p, ok := platform.ByID(platform.ID(strings.ToLower(raw)))
	if !ok || p.ID == platform.Unknown {
		return platform.Platform{}, true, fmt.Errorf("unsupported LOOMLOOM_PLATFORM %q; use shengsuanyun or cogfoundry", raw)
	}
	return p, true, nil
}

func validateTokenPlatform(opts *rootOptions) error {
	resolved, err := resolvePlatform(opts)
	if err != nil {
		return err
	}
	inferred := platform.InferFromServer(opts.server)
	if resolved.ID != platform.Unknown && !resolved.Operational {
		return errors.New(cogFoundryUnavailableMessage)
	}
	if inferred.ID != platform.Unknown && !inferred.Operational {
		return errors.New(cogFoundryUnavailableMessage)
	}
	if strings.TrimSpace(opts.token) == "" || inferred.ID == platform.Unknown {
		return nil
	}
	state := platform.LoadState()
	bound, ok := platform.ByID(state.Platform)
	if !ok || bound.ID == platform.Unknown {
		return nil
	}
	if bound.ID != inferred.ID {
		return fmt.Errorf(
			"平台不一致：LOOMLOOM_SERVER 指向 %s，但本机已绑定 %s，已拦截请求；如确实要切换平台，请更新 LOOMLOOM_SERVER 与对应平台的 token",
			inferred.DisplayName,
			bound.DisplayName,
		)
	}
	return nil
}

func maybePersistVerifiedPlatform(opts *rootOptions, verified bool) {
	if !verified {
		return
	}
	inferred := platform.InferFromServer(opts.server)
	if inferred.ID == platform.Unknown || !inferred.Operational {
		return
	}
	state := platform.LoadState()
	if state.Platform == inferred.ID {
		return
	}
	if state.Platform != "" && state.Platform != platform.Unknown && state.Platform != inferred.ID {
		return
	}
	_ = platform.SaveState(platform.State{Platform: inferred.ID})
}

func isAuthenticatedProductPath(meta client.SuccessMeta) bool {
	if !meta.Authed {
		return false
	}
	return strings.Contains(meta.Path, "/loom/v1/users/me/") ||
		strings.Contains(meta.Path, "/loom/v1/creators/me/")
}

func credentialMessageForPlatform(p platform.Platform) (string, string) {
	if p.ID == platform.ShengSuanYun {
		return "missing_token", missingShengSuanYunTokenMessage
	}
	if p.ID == platform.CogFoundry {
		return "cogfoundry_unavailable", cogFoundryUnavailableMessage
	}
	return "choose_platform", choosePlatformMessage
}

func insufficientBalanceMessage(opts *rootOptions) string {
	p, err := resolvePlatform(opts)
	if err == nil && p.ID == platform.ShengSuanYun {
		return insufficientShengSuanYunBalanceMessage
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
