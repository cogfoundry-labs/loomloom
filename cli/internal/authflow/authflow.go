package authflow

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	DefaultTimeout = 5 * time.Minute

	callbackPath        = "/callback"
	codeChallengeMethod = "S256"
)

type Config struct {
	AuthPageURL   string
	AccountAPIURL string
	AppName       string
	Timeout       time.Duration
	HTTPClient    *http.Client
	OpenURL       func(url string) error
	Notify        func(url string)
}

type Result struct {
	APIKey string
}

type callbackOutcome struct {
	code string
	err  error
}

func Login(ctx context.Context, cfg Config) (*Result, error) {
	if strings.TrimSpace(cfg.AuthPageURL) == "" || strings.TrimSpace(cfg.AccountAPIURL) == "" {
		return nil, fmt.Errorf("browser login is not available: missing authorize page or account API URL")
	}
	verifier, err := randomToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate PKCE verifier: %w", err)
	}
	state, err := randomToken(16)
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start local callback listener: %w", err)
	}
	callbackURL := fmt.Sprintf("http://%s%s", listener.Addr().String(), callbackPath)

	authorizeURL, err := buildAuthorizeURL(cfg, callbackURL, challengeFor(verifier), state)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}

	outcomes := make(chan callbackOutcome, 1)
	server := &http.Server{Handler: callbackHandler(state, outcomes)}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if cfg.Notify != nil {
		cfg.Notify(authorizeURL)
	}
	openURL := cfg.OpenURL
	if openURL == nil {
		openURL = openInBrowser
	}
	_ = openURL(authorizeURL)

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	select {
	case outcome := <-outcomes:
		if outcome.err != nil {
			return nil, outcome.err
		}
		return exchangeCode(ctx, cfg, outcome.code, verifier, callbackURL)
	case <-time.After(timeout):
		return nil, fmt.Errorf("timed out after %s waiting for browser login", timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func callbackHandler(expectedState string, outcomes chan<- callbackOutcome) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		outcome := callbackOutcome{code: query.Get("code")}
		switch {
		case query.Get("error") != "":
			outcome.err = fmt.Errorf("authorization was denied: %s", query.Get("error"))
		case query.Get("state") != expectedState:
			outcome.err = fmt.Errorf("authorization response state mismatch; retry the login")
		case outcome.code == "":
			outcome.err = fmt.Errorf("authorization response is missing the code parameter")
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if outcome.err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "<html><body>登录未完成，请关闭此页面并回到终端重试。</body></html>")
		} else {
			_, _ = io.WriteString(w, "<html><body><script>window.close();</script>登录成功！请返回终端继续操作。</body></html>")
		}

		// Deliver only the first callback; ignore stray repeats.
		select {
		case outcomes <- outcome:
		default:
		}
	})
	return mux
}

func buildAuthorizeURL(cfg Config, callbackURL string, challenge string, state string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(cfg.AuthPageURL))
	if err != nil {
		return "", fmt.Errorf("parse authorize page URL: %w", err)
	}
	query := parsed.Query()
	query.Set("callback_url", callbackURL)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", codeChallengeMethod)
	query.Set("state", state)
	if appName := strings.TrimSpace(cfg.AppName); appName != "" {
		query.Set("app_name", appName)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

type keysEnvelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		APIKey string `json:"api_key"`
	} `json:"data"`
}

func exchangeCode(ctx context.Context, cfg Config, code string, verifier string, callbackURL string) (*Result, error) {
	payload, err := json.Marshal(map[string]string{
		"code":          code,
		"code_verifier": verifier,
		"callback_url":  callbackURL,
	})
	if err != nil {
		return nil, fmt.Errorf("encode key exchange request: %w", err)
	}
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.AccountAPIURL), "/") + "/auth/keys"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read key exchange response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("key exchange failed: status=%d", resp.StatusCode)
	}
	var envelope keysEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode key exchange response: %w", err)
	}
	if envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Msg)
		if message == "" {
			message = fmt.Sprintf("server returned code %d", envelope.Code)
		}
		return nil, fmt.Errorf("key exchange failed: %s", message)
	}
	apiKey := strings.TrimSpace(envelope.Data.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("key exchange succeeded but returned an empty API key")
	}
	return &Result{APIKey: apiKey}, nil
}

func challengeFor(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func openInBrowser(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	default:
		return exec.Command("xdg-open", target).Start()
	}
}
