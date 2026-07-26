package authflow

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

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
		_, _ = fmt.Fprint(w, `{"code":0,"msg":"ok","data":{"api_key":"sk-test-key"}}`)
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
	if result.APIKey != "sk-test-key" {
		t.Fatalf("APIKey = %q want sk-test-key", result.APIKey)
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

func TestLoginRejectsStateMismatch(t *testing.T) {
	recorder := &flowRecorder{}
	cfg := Config{
		AuthPageURL:   "https://example.com/auth",
		AccountAPIURL: "https://example.com",
		Timeout:       5 * time.Second,
		OpenURL: browserStub(t, recorder, func(callbackURL, _ string) string {
			return callbackURL + "?code=test-code&state=forged"
		}),
	}
	_, err := Login(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "state mismatch") {
		t.Fatalf("error = %v want state mismatch", err)
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

func TestLoginRequiresConfiguredURLs(t *testing.T) {
	_, err := Login(context.Background(), Config{})
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("error = %v want configuration error", err)
	}
}
