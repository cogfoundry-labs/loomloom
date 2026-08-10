package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/authflow"
	"github.com/cogfoundry-labs/loomloom/src/cli/internal/platform"
	"github.com/spf13/cobra"
)

const loginAppName = "LoomLoom CLI"

func newLoginCmd(opts *rootOptions) *cobra.Command {
	return newLoginCmdWithRunner(opts, authflow.Login)
}

type loginRunner func(context.Context, authflow.Config) (*authflow.Result, error)

func newLoginCmdWithRunner(opts *rootOptions, runLogin loginRunner) *cobra.Command {
	var (
		noBrowser    bool
		loginTimeout time.Duration
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in through the platform website and save the credential",
		RunE: func(cmd *cobra.Command, args []string) error {
			if loginTimeout <= 0 {
				return fmt.Errorf("--login-timeout must be greater than 0")
			}
			server, resolvedPlatform, err := resolveLoginPlatform(cmd, opts)
			if err != nil {
				return err
			}
			if resolvedPlatform.AuthPageURL == "" || resolvedPlatform.AccountAPIURL == "" {
				return fmt.Errorf(
					"platform %s does not support browser login; configure a token instead (see `loomloom doctor`)",
					resolvedPlatform.DisplayName,
				)
			}

			flowConfig := authflow.Config{
				AuthPageURL:          resolvedPlatform.AuthPageURL,
				AccountAPIURL:        resolvedPlatform.AccountAPIURL,
				AppName:              loginAppName,
				CallbackPageVariant:  callbackPageVariantForPlatform(resolvedPlatform.ID),
				AuthorizationTimeout: loginTimeout,
				ExchangeTimeout:      opts.timeout,
				Notify: func(authorizeURL string) {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "请在浏览器中完成登录（如未自动打开，请手动访问）：\n%s\n", authorizeURL)
				},
			}
			if noBrowser {
				flowConfig.OpenURL = func(string) error { return nil }
			}
			result, err := runLogin(cmd.Context(), flowConfig)
			if err != nil {
				return err
			}

			opts.server = server
			opts.token = result.Token
			if err := verifyLoginToken(cmd, opts); err != nil {
				return err
			}

			state := platform.LoadState()
			profile, err := state.UpsertVerified(server, resolvedPlatform.ID, "", time.Now())
			if err != nil {
				return fmt.Errorf("save login server profile: %w", err)
			}
			profile, err = state.SetToken(profile.Name, result.Token)
			if err != nil {
				return fmt.Errorf("save login credentials: %w", err)
			}
			if err := platform.SaveState(state); err != nil {
				return fmt.Errorf("save login credentials: %w", err)
			}

			if env := strings.TrimSpace(os.Getenv(profile.TokenEnv)); env != "" && env != result.Token {
				_, _ = fmt.Fprintln(
					cmd.ErrOrStderr(),
					"警告：当前服务器的 Token 环境变量已设置且与本次登录凭据不同；环境变量优先级更高，如需使用本次登录结果请先取消对应变量。",
				)
			}

			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"server":        server,
					"platform":      string(resolvedPlatform.ID),
					"platform_name": resolvedPlatform.DisplayName,
					"token_saved":   true,
					"token":         maskToken(result.Token),
					"message":       "ok",
				})
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "登录成功：%s\n", resolvedPlatform.DisplayName)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "server: %s\n", server)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "token: %s（已保存，可直接使用其他命令）\n", maskToken(result.Token))
			return nil
		},
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the login URL instead of opening a browser")
	cmd.Flags().DurationVar(
		&loginTimeout,
		"login-timeout",
		authflow.DefaultAuthorizationTimeout,
		"Maximum time to wait for browser authorization",
	)
	return cmd
}

func callbackPageVariantForPlatform(platformID platform.ID) authflow.CallbackPageVariant {
	switch platformID {
	case platform.CogFoundry:
		return authflow.CallbackPageCogFoundry
	case platform.ShengSuanYun:
		return authflow.CallbackPageShengSuanYun
	default:
		return authflow.CallbackPageGeneric
	}
}

func newLogoutCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the browser login credential saved by `loomloom login`",
		RunE: func(cmd *cobra.Command, args []string) error {
			state := platform.LoadState()
			profile, ok := state.FindProfile(opts.server)
			hadToken := ok && strings.TrimSpace(profile.Token) != ""
			environmentTokenSet := ok && strings.TrimSpace(os.Getenv(profile.TokenEnv)) != ""
			if hadToken {
				var err error
				profile, err = state.SetToken(profile.Name, "")
				if err != nil {
					return err
				}
				if err := platform.SaveState(state); err != nil {
					return fmt.Errorf("clear login credentials: %w", err)
				}
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"server":                profile.Server,
					"platform":              string(profile.Platform),
					"token_removed":         hadToken,
					"environment_token_set": environmentTokenSet,
				})
			}
			if hadToken {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "浏览器登录凭据已删除。")
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "当前没有保存的浏览器登录凭据，无需退出。")
			}
			if environmentTokenSet {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "当前仍检测到环境变量 API Token；该变量未被删除，CLI 后续仍会优先使用它。")
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "当前未检测到环境变量 API Token。")
			}
			return nil
		},
	}
}

func verifyLoginToken(cmd *cobra.Command, opts *rootOptions) error {
	httpClient, err := newHTTPClientForDoctor(opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
	defer cancel()
	query := url.Values{}
	query.Set("pageSize", "1")
	var probeResp map[string]any
	if err := httpClient.GetProductJSONWithQuery(ctx, "/users/me/executables", query, &probeResp); err != nil {
		if isAuthenticationFailure(err) {
			return fmt.Errorf("登录获取的凭据未通过当前 Server 验证，请确认 Server 与登录平台一致后重试")
		}
		return fmt.Errorf("verify login token against %s: %w", opts.server, err)
	}
	return nil
}

func maskToken(token string) string {
	if len(token) <= 10 {
		return "****"
	}
	return token[:6] + "..." + token[len(token)-4:]
}
