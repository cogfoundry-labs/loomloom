package authflow

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type flowRecorder struct {
	mu           sync.Mutex
	challenge    string
	method       string
	state        string
	appName      string
	verifier     string
	code         string
	callbackURL  string
	exchangePath string
}

func newAccountAPI(t *testing.T, recorder *flowRecorder, respond func(w http.ResponseWriter)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode exchange body: %v", err)
		}
		recorder.mu.Lock()
		recorder.exchangePath = r.URL.Path
		recorder.verifier = body["code_verifier"]
		recorder.code = body["code"]
		recorder.callbackURL = body["callback_url"]
		recorder.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		respond(w)
	}))
}

func browserStub(t *testing.T, recorder *flowRecorder, callbackQuery func(callbackURL string, state string) string) func(string) error {
	t.Helper()
	return func(authorizeURL string) error {
		parsed, err := url.Parse(authorizeURL)
		if err != nil {
			return err
		}
		query := parsed.Query()
		recorder.mu.Lock()
		recorder.challenge = query.Get("code_challenge")
		recorder.method = query.Get("code_challenge_method")
		recorder.state = query.Get("state")
		recorder.appName = query.Get("app_name")
		recorder.mu.Unlock()
		target := callbackQuery(query.Get("callback_url"), query.Get("state"))
		go func() {
			resp, err := http.Get(target)
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
		return nil
	}
}

func TestLoginHappyPath(t *testing.T) {
	recorder := &flowRecorder{}
	accountAPI := newAccountAPI(t, recorder, func(w http.ResponseWriter) {
		_, _ = fmt.Fprint(w, `{"code":0,"msg":"ok","data":{"api_key":"sk-test-key","jwt_token":"signed-jwt"}}`)
	})
	defer accountAPI.Close()

	cfg := Config{
		AuthPageURL:   "https://example.com/auth",
		AccountAPIURL: accountAPI.URL,
		AppName:       "LoomLoom CLI",
		Timeout:       5 * time.Second,
		OpenURL: browserStub(t, recorder, func(callbackURL, state string) string {
			return callbackURL + "?code=test-code&state=" + url.QueryEscape(state)
		}),
	}
	result, err := Login(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.Token != "sk-test-key" {
		t.Fatalf("Token = %q want sk-test-key", result.Token)
	}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if recorder.exchangePath != "/auth/keys" {
		t.Fatalf("exchange path = %q want /auth/keys", recorder.exchangePath)
	}
	if recorder.code != "test-code" {
		t.Fatalf("exchanged code = %q want test-code", recorder.code)
	}
	if recorder.method != codeChallengeMethod {
		t.Fatalf("code_challenge_method = %q want %s", recorder.method, codeChallengeMethod)
	}
	if recorder.appName != "LoomLoom CLI" {
		t.Fatalf("app_name = %q want LoomLoom CLI", recorder.appName)
	}
	if !strings.HasPrefix(recorder.callbackURL, "http://127.0.0.1:") {
		t.Fatalf("callback_url = %q want loopback", recorder.callbackURL)
	}
	sum := sha256.Sum256([]byte(recorder.verifier))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != recorder.challenge {
		t.Fatal("code_verifier does not match the code_challenge sent to the authorize page")
	}
}

func TestLoginUsesConfiguredCallbackPageVariant(t *testing.T) {
	tests := []struct {
		name         string
		variant      CallbackPageVariant
		wantLanguage string
		wantText     string
		unwantedText string
	}{
		{
			name:         "CogFoundry",
			variant:      CallbackPageCogFoundry,
			wantLanguage: "en",
			wantText:     "CogFoundry has sent the authorization response",
			unwantedText: "胜算云",
		},
		{
			name:         "ShengSuanYun",
			variant:      CallbackPageShengSuanYun,
			wantLanguage: "zh-CN",
			wantText:     "胜算云已将授权响应发送给 LoomLoom CLI",
			unwantedText: "CogFoundry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &flowRecorder{}
			accountAPI := newAccountAPI(t, recorder, func(w http.ResponseWriter) {
				_, _ = fmt.Fprint(w, `{"code":0,"msg":"ok","data":{"api_key":"sk-test-key"}}`)
			})
			defer accountAPI.Close()

			var callbackBody string
			var callbackLanguage string
			cfg := Config{
				AuthPageURL:         "https://example.com/auth",
				AccountAPIURL:       accountAPI.URL,
				CallbackPageVariant: tt.variant,
				Timeout:             5 * time.Second,
				OpenURL: func(authorizeURL string) error {
					parsed, err := url.Parse(authorizeURL)
					if err != nil {
						return err
					}
					query := parsed.Query()
					target := query.Get("callback_url") + "?code=test-code&state=" + url.QueryEscape(query.Get("state"))
					response, err := http.Get(target)
					if err != nil {
						return err
					}
					defer func() { _ = response.Body.Close() }()
					body, err := io.ReadAll(response.Body)
					if err != nil {
						return err
					}
					callbackBody = string(body)
					callbackLanguage = response.Header.Get("Content-Language")
					return nil
				},
			}

			if _, err := Login(context.Background(), cfg); err != nil {
				t.Fatalf("Login() error = %v", err)
			}
			if callbackLanguage != tt.wantLanguage {
				t.Errorf("callback language = %q want %q", callbackLanguage, tt.wantLanguage)
			}
			if !strings.Contains(callbackBody, tt.wantText) {
				t.Errorf("callback page does not contain %q", tt.wantText)
			}
			if strings.Contains(callbackBody, tt.unwantedText) {
				t.Errorf("callback page unexpectedly contains %q", tt.unwantedText)
			}
		})
	}
}

func TestLoginAcceptsAPIKeyFromOlderAccountAPI(t *testing.T) {
	recorder := &flowRecorder{}
	accountAPI := newAccountAPI(t, recorder, func(w http.ResponseWriter) {
		_, _ = fmt.Fprint(w, `{"code":0,"msg":"ok","data":{"api_key":"sk-legacy-key"}}`)
	})
	defer accountAPI.Close()

	result, err := exchangeCode(
		context.Background(),
		Config{AccountAPIURL: accountAPI.URL},
		"test-code",
		"test-verifier",
		"http://127.0.0.1:43127/callback",
	)
	if err != nil {
		t.Fatalf("exchangeCode() error = %v", err)
	}
	if result.Token != "sk-legacy-key" {
		t.Fatalf("Token = %q want sk-legacy-key", result.Token)
	}
}

func TestLoginFallsBackToJWTWhenAPIKeyIsUnavailable(t *testing.T) {
	recorder := &flowRecorder{}
	accountAPI := newAccountAPI(t, recorder, func(w http.ResponseWriter) {
		_, _ = fmt.Fprint(w, `{"code":0,"msg":"ok","data":{"jwt_token":"signed-jwt"}}`)
	})
	defer accountAPI.Close()

	result, err := exchangeCode(
		context.Background(),
		Config{AccountAPIURL: accountAPI.URL},
		"test-code",
		"test-verifier",
		"http://127.0.0.1:43127/callback",
	)
	if err != nil {
		t.Fatalf("exchangeCode() error = %v", err)
	}
	if result.Token != "signed-jwt" {
		t.Fatalf("Token = %q want signed-jwt", result.Token)
	}
}

func TestLoginIgnoresStateMismatchAndAcceptsValidCallback(t *testing.T) {
	recorder := &flowRecorder{}
	accountAPI := newAccountAPI(t, recorder, func(w http.ResponseWriter) {
		_, _ = fmt.Fprint(w, `{"code":0,"msg":"ok","data":{"api_key":"sk-test-key"}}`)
	})
	defer accountAPI.Close()

	cfg := Config{
		AuthPageURL:   "https://example.com/auth",
		AccountAPIURL: accountAPI.URL,
		Timeout:       5 * time.Second,
		OpenURL: func(authorizeURL string) error {
			parsed, err := url.Parse(authorizeURL)
			if err != nil {
				return err
			}
			query := parsed.Query()
			callbackURL := query.Get("callback_url")
			forgedResponse, err := http.Get(callbackURL + "?code=forged-code&state=forged-state")
			if err != nil {
				return err
			}
			_ = forgedResponse.Body.Close()
			if forgedResponse.StatusCode != http.StatusBadRequest {
				return fmt.Errorf("forged callback status = %d", forgedResponse.StatusCode)
			}

			validTarget := callbackURL + "?code=test-code&state=" + url.QueryEscape(query.Get("state"))
			validResponse, err := http.Get(validTarget)
			if err != nil {
				return err
			}
			return validResponse.Body.Close()
		},
	}
	result, err := Login(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.Token != "sk-test-key" {
		t.Fatalf("Token = %q want sk-test-key", result.Token)
	}
}

func TestLoginRejectsDeniedAuthorization(t *testing.T) {
	recorder := &flowRecorder{}
	cfg := Config{
		AuthPageURL:   "https://example.com/auth",
		AccountAPIURL: "https://example.com",
		Timeout:       5 * time.Second,
		OpenURL: browserStub(t, recorder, func(callbackURL, state string) string {
			return callbackURL + "?error=access_denied&state=" + url.QueryEscape(state)
		}),
	}
	_, err := Login(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("error = %v want denied", err)
	}
}

func TestCallbackPageUsesStrictHeadersAndDoesNotReflectErrors(t *testing.T) {
	outcomes := make(chan callbackOutcome, 1)
	handler := callbackHandler("expected-state", CallbackPageCogFoundry, outcomes)
	request := httptest.NewRequest(
		http.MethodGet,
		"/callback?error=%1B%5B31mforged&state=expected-state",
		nil,
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d want %d", response.Code, http.StatusBadRequest)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q want no-store", got)
	}
	if got := response.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'none'") {
		t.Fatalf("Content-Security-Policy = %q want strict default", got)
	}
	if strings.Contains(response.Body.String(), "forged") {
		t.Fatal("callback page must not reflect the error query parameter")
	}
	if !strings.Contains(response.Body.String(), "Authorization not completed") {
		t.Fatal("callback page must explain that authorization did not complete")
	}

	select {
	case outcome := <-outcomes:
		if outcome.err == nil || !strings.Contains(outcome.err.Error(), "denied") {
			t.Fatalf("outcome error = %v want denied", outcome.err)
		}
	default:
		t.Fatal("callback handler did not deliver an outcome")
	}
}

func TestCallbackHandlerDeliversOnlyTheFirstOutcome(t *testing.T) {
	outcomes := make(chan callbackOutcome, 1)
	handler := callbackHandler("expected-state", CallbackPageCogFoundry, outcomes)

	first := httptest.NewRecorder()
	handler.ServeHTTP(
		first,
		httptest.NewRequest(http.MethodGet, "/callback?code=first&state=expected-state", nil),
	)
	if outcome := <-outcomes; outcome.err != nil || outcome.code != "first" {
		t.Fatalf("first outcome = %+v want code=first", outcome)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(
		second,
		httptest.NewRequest(http.MethodGet, "/callback?code=second&state=expected-state", nil),
	)
	select {
	case outcome := <-outcomes:
		t.Fatalf("unexpected repeated outcome = %+v", outcome)
	default:
	}
}

func TestLoginSurfacesExchangeFailure(t *testing.T) {
	recorder := &flowRecorder{}
	accountAPI := newAccountAPI(t, recorder, func(w http.ResponseWriter) {
		_, _ = fmt.Fprint(w, `{"code":503,"msg":"Invalid or expired authorization code","data":null}`)
	})
	defer accountAPI.Close()

	cfg := Config{
		AuthPageURL:   "https://example.com/auth",
		AccountAPIURL: accountAPI.URL,
		Timeout:       5 * time.Second,
		OpenURL: browserStub(t, recorder, func(callbackURL, state string) string {
			return callbackURL + "?code=stale-code&state=" + url.QueryEscape(state)
		}),
	}
	_, err := Login(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "Invalid or expired authorization code") {
		t.Fatalf("error = %v want exchange failure message", err)
	}
}

func TestResolveTimeouts(t *testing.T) {
	tests := []struct {
		name             string
		config           Config
		wantAuthorize    time.Duration
		wantExchange     time.Duration
		wantErrorMessage string
	}{
		{
			name:          "defaults",
			wantAuthorize: DefaultAuthorizationTimeout,
			wantExchange:  DefaultExchangeTimeout,
		},
		{
			name:          "legacy authorization timeout",
			config:        Config{Timeout: 2 * time.Minute},
			wantAuthorize: 2 * time.Minute,
			wantExchange:  DefaultExchangeTimeout,
		},
		{
			name: "explicit values",
			config: Config{
				AuthorizationTimeout: 3 * time.Minute,
				ExchangeTimeout:      45 * time.Second,
				Timeout:              2 * time.Minute,
			},
			wantAuthorize: 3 * time.Minute,
			wantExchange:  45 * time.Second,
		},
		{
			name:             "negative authorization timeout",
			config:           Config{AuthorizationTimeout: -time.Second},
			wantErrorMessage: "authorization timeout must be greater than 0",
		},
		{
			name:             "negative exchange timeout",
			config:           Config{ExchangeTimeout: -time.Second},
			wantErrorMessage: "exchange timeout must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorizationTimeout, exchangeTimeout, err := resolveTimeouts(tt.config)
			if tt.wantErrorMessage != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrorMessage) {
					t.Fatalf("error = %v want %q", err, tt.wantErrorMessage)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveTimeouts() error = %v", err)
			}
			if authorizationTimeout != tt.wantAuthorize || exchangeTimeout != tt.wantExchange {
				t.Fatalf(
					"timeouts = (%s, %s) want (%s, %s)",
					authorizationTimeout,
					exchangeTimeout,
					tt.wantAuthorize,
					tt.wantExchange,
				)
			}
		})
	}
}

func TestLoginTimesOutWaitingForBrowserAuthorization(t *testing.T) {
	var (
		callbackURL    string
		exchangeCalled bool
	)
	cfg := Config{
		AuthPageURL:          "https://example.com/auth",
		AccountAPIURL:        "https://example.com",
		AuthorizationTimeout: 10 * time.Millisecond,
		ExchangeTimeout:      time.Second,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			exchangeCalled = true
			return nil, fmt.Errorf("unexpected exchange")
		})},
		OpenURL: func(authorizeURL string) error {
			parsed, err := url.Parse(authorizeURL)
			if err != nil {
				return err
			}
			callbackURL = parsed.Query().Get("callback_url")
			return nil
		},
	}

	result, err := Login(context.Background(), cfg)
	if result != nil {
		t.Fatalf("result = %+v want nil", result)
	}
	if !errors.Is(err, ErrAuthorizationTimeout) {
		t.Fatalf("error = %v want ErrAuthorizationTimeout", err)
	}
	if !strings.Contains(err.Error(), "10ms") || !strings.Contains(err.Error(), "--login-timeout") {
		t.Fatalf("error = %v want duration and recovery guidance", err)
	}
	if exchangeCalled {
		t.Fatal("authorization code exchange must not run before a callback")
	}
	assertCallbackListenerClosed(t, callbackURL)
}

func TestLoginTimesOutExchangingAuthorizationCode(t *testing.T) {
	transportDone := make(chan struct{})
	var callbackURL string
	cfg := Config{
		AuthPageURL:          "https://example.com/auth",
		AccountAPIURL:        "https://example.com",
		AuthorizationTimeout: time.Second,
		ExchangeTimeout:      10 * time.Millisecond,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if _, ok := request.Context().Deadline(); !ok {
				return nil, fmt.Errorf("exchange request is missing a deadline")
			}
			<-request.Context().Done()
			close(transportDone)
			return nil, request.Context().Err()
		})},
		OpenURL: func(authorizeURL string) error {
			parsed, err := url.Parse(authorizeURL)
			if err != nil {
				return err
			}
			query := parsed.Query()
			callbackURL = query.Get("callback_url")
			target := callbackURL + "?code=test-code&state=" + url.QueryEscape(query.Get("state"))
			response, err := http.Get(target)
			if err != nil {
				return err
			}
			return response.Body.Close()
		},
	}

	result, err := Login(context.Background(), cfg)
	if result != nil {
		t.Fatalf("result = %+v want nil", result)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v want context deadline exceeded", err)
	}
	if !strings.Contains(err.Error(), "10ms") || !strings.Contains(err.Error(), "exchanging authorization code") {
		t.Fatalf("error = %v want exchange timeout details", err)
	}
	select {
	case <-transportDone:
	default:
		t.Fatal("exchange transport did not stop after its context timed out")
	}
	assertCallbackListenerClosed(t, callbackURL)
}

func TestLoginPreservesParentCancellationDuringExchange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	transportStarted := make(chan struct{})
	transportDone := make(chan struct{})
	cfg := Config{
		AuthPageURL:          "https://example.com/auth",
		AccountAPIURL:        "https://example.com",
		AuthorizationTimeout: time.Second,
		ExchangeTimeout:      time.Minute,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			close(transportStarted)
			<-request.Context().Done()
			close(transportDone)
			return nil, request.Context().Err()
		})},
		OpenURL: func(authorizeURL string) error {
			parsed, err := url.Parse(authorizeURL)
			if err != nil {
				return err
			}
			query := parsed.Query()
			target := query.Get("callback_url") + "?code=test-code&state=" + url.QueryEscape(query.Get("state"))
			response, err := http.Get(target)
			if err != nil {
				return err
			}
			return response.Body.Close()
		},
	}

	result := make(chan error, 1)
	go func() {
		_, err := Login(ctx, cfg)
		result <- err
	}()
	select {
	case <-transportStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("exchange did not start")
	}
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v want context canceled", err)
		}
		if strings.Contains(err.Error(), "timed out") {
			t.Fatalf("parent cancellation was misreported as a timeout: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Login() did not return after parent cancellation")
	}
	select {
	case <-transportDone:
	default:
		t.Fatal("exchange transport did not stop after parent cancellation")
	}
}

func TestLoginReturnsBeforeSideEffectsWhenContextIsAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	openCalled := false
	notifyCalled := false
	_, err := Login(ctx, Config{
		AuthPageURL:   "https://example.com/auth",
		AccountAPIURL: "https://example.com",
		OpenURL: func(string) error {
			openCalled = true
			return nil
		},
		Notify: func(string) {
			notifyCalled = true
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v want context canceled", err)
	}
	if openCalled || notifyCalled {
		t.Fatalf("side effects = open:%t notify:%t want none", openCalled, notifyCalled)
	}
}

func TestLoginPreservesParentCancellationWhileWaitingForAuthorization(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var callbackURL string
	_, err := Login(ctx, Config{
		AuthPageURL:          "https://example.com/auth",
		AccountAPIURL:        "https://example.com",
		AuthorizationTimeout: time.Nanosecond,
		OpenURL: func(authorizeURL string) error {
			parsed, parseErr := url.Parse(authorizeURL)
			if parseErr != nil {
				return parseErr
			}
			callbackURL = parsed.Query().Get("callback_url")
			cancel()
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v want context canceled", err)
	}
	if errors.Is(err, ErrAuthorizationTimeout) {
		t.Fatalf("parent cancellation was misreported as authorization timeout: %v", err)
	}
	assertCallbackListenerClosed(t, callbackURL)
}

func TestExchangeCodeRejectsOversizedResponse(t *testing.T) {
	cfg := Config{
		AccountAPIURL: "https://example.com",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", maxExchangeResponseLength+1))),
			}, nil
		})},
	}

	_, err := exchangeCode(
		context.Background(),
		cfg,
		"test-code",
		"test-verifier",
		"http://127.0.0.1:43127/callback",
	)
	if err == nil || !strings.Contains(err.Error(), "response exceeds") {
		t.Fatalf("error = %v want response size rejection", err)
	}
}

func TestExchangeHTTPClientRejectsRedirectsWithoutMutatingConfiguredClient(t *testing.T) {
	original := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return nil }}
	client := exchangeHTTPClient(original)
	if client == original {
		t.Fatal("exchangeHTTPClient() must clone the configured client")
	}
	if err := client.CheckRedirect(&http.Request{}, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("exchange redirect error = %v want http.ErrUseLastResponse", err)
	}
	if err := original.CheckRedirect(&http.Request{}, nil); err != nil {
		t.Fatalf("configured client was mutated: %v", err)
	}
}

func assertCallbackListenerClosed(t *testing.T, callbackURL string) {
	t.Helper()
	parsed, err := url.Parse(callbackURL)
	if err != nil {
		t.Fatalf("parse callback URL: %v", err)
	}
	connection, err := net.DialTimeout("tcp", parsed.Host, 100*time.Millisecond)
	if err == nil {
		_ = connection.Close()
		t.Fatalf("callback listener %s is still accepting connections", parsed.Host)
	}
}

func TestLoginRequiresConfiguredURLs(t *testing.T) {
	_, err := Login(context.Background(), Config{})
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("error = %v want configuration error", err)
	}
}
