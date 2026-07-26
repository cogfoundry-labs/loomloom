package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/authflow"
	"github.com/Cogfoundry-ai/loomloom/cli/internal/platform"
	"github.com/spf13/cobra"
)

const loginAppName = "LoomLoom CLI"

func newLoginCmd(opts *rootOptions) *cobra.Command {
	var noBrowser bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in through the platform website and save the API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			server, resolvedPlatform := loginTarget(opts)
			if resolvedPlatform.AuthPageURL == "" || resolvedPlatform.AccountAPIURL == "" {
				return fmt.Errorf(
					"platform %s does not support browser login; configure a token instead (see `loomloom doctor`)",
					resolvedPlatform.DisplayName,
				)
			}

			flowConfig := authflow.Config{
				AuthPageURL:   resolvedPlatform.AuthPageURL,
				AccountAPIURL: resolvedPlatform.AccountAPIURL,
				AppName:       loginAppName,
				Notify: func(authorizeURL string) {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "请在浏览器中完成登录（如未自动打开，请手动访问）：\n%s\n", authorizeURL)
				},
			}
			if noBrowser {
				flowConfig.OpenURL = func(string) error { return nil }
			}
			result, err := authflow.Login(cmd.Context(), flowConfig)
			if err != nil {
				return err
			}

			opts.server = server
			opts.token = result.APIKey
			if err := verifyLoginToken(cmd, opts); err != nil {
				return err
			}

			state := platform.LoadState()
			profile, err := state.UpsertVerified(server, resolvedPlatform.ID, "", time.Now())
			if err != nil {
				return fmt.Errorf("save login server profile: %w", err)
			}
			profile, err = state.SetToken(profile.Name, result.APIKey)
			if err != nil {
				return fmt.Errorf("save login credentials: %w", err)
			}
			if err := platform.SaveState(state); err != nil {
				return fmt.Errorf("save login credentials: %w", err)
			}

			if env := strings.TrimSpace(os.Getenv(profile.TokenEnv)); env != "" && env != result.APIKey {
				_, _ = fmt.Fprintln(
					cmd.ErrOrStderr(),
					"警告：当前服务器的 Token 环境变量已设置且与本次登录的密钥不同；环境变量优先级更高，如需使用本次登录结果请先取消对应变量。",
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
					"token":         maskToken(result.APIKey),
					"message":       "ok",
				})
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "登录成功：%s\n", resolvedPlatform.DisplayName)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "server: %s\n", server)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "token: %s（已保存，可直接使用其他命令）\n", maskToken(result.APIKey))
			return nil
		},
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the login URL instead of opening a browser")
	return cmd
}

func newLogoutCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the API key saved by `loomloom login`",
		RunE: func(cmd *cobra.Command, args []string) error {
			state := platform.LoadState()
			profile, ok := state.FindProfile(opts.server)
			if !ok {
				return fmt.Errorf("no saved login found; run `loomloom login` first")
			}
			hadToken := strings.TrimSpace(profile.Token) != ""
			profile, err := state.SetToken(profile.Name, "")
			if err != nil {
				return err
			}
			if err := platform.SaveState(state); err != nil {
				return fmt.Errorf("clear login credentials: %w", err)
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"server":        profile.Server,
					"platform":      string(profile.Platform),
					"token_removed": hadToken,
				})
			}
			if hadToken {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "已退出登录（保存的密钥已删除）。")
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "当前没有保存的密钥，无需退出。")
			}
			return nil
		},
	}
}

func loginTarget(opts *rootOptions) (string, platform.Platform) {
	server := strings.TrimSpace(opts.server)
	if server == "" {
		fallback, _ := platform.ByID(platform.ShengSuanYun)
		return fallback.DefaultServer, fallback
	}
	return server, platform.InferFromServer(server)
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
			return fmt.Errorf("登录获取的密钥未通过当前 Server 验证，请确认 Server 与登录平台一致后重试")
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
