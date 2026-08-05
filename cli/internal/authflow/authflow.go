package authflow

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	DefaultAuthorizationTimeout = 5 * time.Minute
	DefaultExchangeTimeout      = 30 * time.Second
	DefaultTimeout              = DefaultAuthorizationTimeout

	callbackPath              = "/callback"
	codeChallengeMethod       = "S256"
	callbackServerTimeout     = 5 * time.Second
	callbackShutdownTimeout   = time.Second
	maxExchangeResponseLength = 64 << 10
)

var ErrAuthorizationTimeout = errors.New("browser authorization timed out")

type Config struct {
	AuthPageURL          string
	AccountAPIURL        string
	AppName              string
	CallbackPageVariant  CallbackPageVariant
	AuthorizationTimeout time.Duration
	ExchangeTimeout      time.Duration
	HTTPClient           *http.Client
	OpenURL              func(url string) error
	Notify               func(url string)
	Timeout              time.Duration
}

type Result struct {
	Token string
}

type callbackOutcome struct {
	code string
	err  error
}

func Login(ctx context.Context, cfg Config) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.AuthPageURL) == "" || strings.TrimSpace(cfg.AccountAPIURL) == "" {
		return nil, fmt.Errorf("browser login is not available: missing authorize page or account API URL")
	}
	authorizationTimeout, exchangeTimeout, err := resolveTimeouts(cfg)
	if err != nil {
		return nil, err
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
	server := &http.Server{
		Handler:           callbackHandler(state, cfg.CallbackPageVariant, outcomes),
		ReadHeaderTimeout: callbackServerTimeout,
		WriteTimeout:      callbackServerTimeout,
		IdleTimeout:       callbackServerTimeout,
		MaxHeaderBytes:    8 << 10,
	}
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.Serve(listener)
		close(serveResult)
	}()
	var stopServerOnce sync.Once
	stopServer := func() {
		stopServerOnce.Do(func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), callbackShutdownTimeout)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				_ = server.Close()
			}
			<-serveResult
		})
	}
	defer stopServer()

	if cfg.Notify != nil {
		cfg.Notify(authorizeURL)
	}
	openURL := cfg.OpenURL
	if openURL == nil {
		openURL = openInBrowser
	}
	_ = openURL(authorizeURL)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	timer := time.NewTimer(authorizationTimeout)
	defer timer.Stop()
	select {
	case outcome := <-outcomes:
		stopServer()
		if outcome.err != nil {
			return nil, outcome.err
		}
		return exchangeCodeWithTimeout(ctx, cfg, exchangeTimeout, outcome.code, verifier, callbackURL)
	case <-timer.C:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w after %s; run login again or increase --login-timeout", ErrAuthorizationTimeout, authorizationTimeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	case serveErr := <-serveResult:
		if errors.Is(serveErr, http.ErrServerClosed) {
			return nil, fmt.Errorf("local callback server stopped before authorization completed")
		}
		return nil, fmt.Errorf("serve local authorization callback: %w", serveErr)
	}
}

func resolveTimeouts(cfg Config) (time.Duration, time.Duration, error) {
	authorizationTimeout := cfg.AuthorizationTimeout
	if authorizationTimeout == 0 {
		authorizationTimeout = cfg.Timeout
	}
	if authorizationTimeout < 0 {
		return 0, 0, fmt.Errorf("authorization timeout must be greater than 0")
	}
	if authorizationTimeout == 0 {
		authorizationTimeout = DefaultAuthorizationTimeout
	}

	exchangeTimeout := cfg.ExchangeTimeout
	if exchangeTimeout < 0 {
		return 0, 0, fmt.Errorf("exchange timeout must be greater than 0")
	}
	if exchangeTimeout == 0 {
		exchangeTimeout = DefaultExchangeTimeout
	}
	return authorizationTimeout, exchangeTimeout, nil
}

func exchangeCodeWithTimeout(
	ctx context.Context,
	cfg Config,
	timeout time.Duration,
	code string,
	verifier string,
	callbackURL string,
) (*Result, error) {
	exchangeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := exchangeCode(exchangeCtx, cfg, code, verifier, callbackURL)
	if err == nil {
		return result, nil
	}
	if parentErr := ctx.Err(); parentErr != nil {
		return nil, parentErr
	}
	if errors.Is(exchangeCtx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("timed out after %s exchanging authorization code: %w", timeout, context.DeadlineExceeded)
	}
	return nil, err
}

func callbackHandler(expectedState string, pageVariant CallbackPageVariant, outcomes chan<- callbackOutcome) http.Handler {
	mux := http.NewServeMux()
	var deliverOutcome sync.Once
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		query := r.URL.Query()
		states := query["state"]
		codes := query["code"]
		errors := query["error"]
		if len(states) != 1 || states[0] != expectedState {
			writeCallbackPage(w, pageVariant, false)
			return
		}
		outcome := callbackOutcome{}
		switch {
		case len(errors) == 1 && errors[0] != "" && len(codes) == 0:
			outcome.err = fmt.Errorf("authorization was denied")
		case len(errors) > 0:
			outcome.err = fmt.Errorf("authorization response contains conflicting error parameters")
		case len(codes) != 1 || strings.TrimSpace(codes[0]) == "":
			outcome.err = fmt.Errorf("authorization response is missing the code parameter")
		default:
			outcome.code = codes[0]
		}

		writeCallbackPage(w, pageVariant, outcome.err == nil)
		deliverOutcome.Do(func() { outcomes <- outcome })
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
		APIKey   string `json:"api_key"`
		JWTToken string `json:"jwt_token"`
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

	httpClient := exchangeHTTPClient(cfg.HTTPClient)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxExchangeResponseLength+1))
	if err != nil {
		return nil, fmt.Errorf("read key exchange response: %w", err)
	}
	if len(body) > maxExchangeResponseLength {
		return nil, fmt.Errorf("key exchange response exceeds %d bytes", maxExchangeResponseLength)
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
	token := strings.TrimSpace(envelope.Data.APIKey)
	if token == "" {
		token = strings.TrimSpace(envelope.Data.JWTToken)
	}
	if token == "" {
		return nil, fmt.Errorf("key exchange succeeded but returned an empty credential")
	}
	return &Result{Token: token}, nil
}

func exchangeHTTPClient(configured *http.Client) *http.Client {
	if configured == nil {
		configured = http.DefaultClient
	}
	client := *configured
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &client
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
