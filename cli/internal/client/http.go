package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/platform"
)

type Config struct {
	BaseURL   string
	Token     string
	Timeout   time.Duration
	Verbose   bool
	LogWriter io.Writer
	OnSuccess func(SuccessMeta)
}

type SuccessMeta struct {
	Method string
	Path   string
	Authed bool
}

type RequestError struct {
	StatusCode int
	Body       string
}

func (e RequestError) Error() string {
	return fmt.Sprintf("request failed: status=%d body=%s", e.StatusCode, e.Body)
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	verbose    bool
	logWriter  io.Writer
	onSuccess  func(SuccessMeta)
}

func New(cfg Config) (*Client, error) {
	baseURL, err := normalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	logWriter := cfg.LogWriter
	if logWriter == nil {
		logWriter = io.Discard
	}
	return &Client{
		baseURL: baseURL,
		token:   strings.TrimSpace(cfg.Token),
		httpClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) == 0 {
					return nil
				}
				origin := via[0].URL
				if !strings.EqualFold(origin.Scheme, req.URL.Scheme) ||
					!strings.EqualFold(origin.Host, req.URL.Host) {
					return fmt.Errorf("refusing cross-server redirect from %s://%s to %s://%s", origin.Scheme, origin.Host, req.URL.Scheme, req.URL.Host)
				}
				return nil
			},
		},
		verbose:   cfg.Verbose,
		logWriter: logWriter,
		onSuccess: cfg.OnSuccess,
	}, nil
}

func normalizeBaseURL(raw string) (string, error) {
	return platform.NormalizeServer(raw)
}

func (c *Client) endpoint(path string) string {
	path = "/" + strings.TrimLeft(path, "/")
	return c.baseURL + path
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	startedAt := time.Now()
	c.logf("%s %s", req.Method, req.URL.EscapedPath())
	resp, err := c.httpClient.Do(req)
	duration := time.Since(startedAt).Round(time.Millisecond)
	if err != nil {
		c.logf("request failed method=%s path=%s duration=%s error=%s", req.Method, req.URL.EscapedPath(), duration, transportErrorKind(err))
		return nil, err
	}
	requestID := responseRequestID(resp)
	if requestID == "" {
		c.logf("response status=%d duration=%s", resp.StatusCode, duration)
	} else {
		c.logf("response status=%d duration=%s request_id=%s", resp.StatusCode, duration, requestID)
	}
	return resp, nil
}

func (c *Client) notifySuccess(req *http.Request) {
	if c.onSuccess == nil {
		return
	}
	c.onSuccess(SuccessMeta{
		Method: req.Method,
		Path:   req.URL.EscapedPath(),
		Authed: strings.TrimSpace(req.Header.Get("Authorization")) != "",
	})
}

func (c *Client) logf(format string, args ...any) {
	if !c.verbose {
		return
	}
	_, _ = fmt.Fprintf(c.logWriter, "[debug] "+format+"\n", args...)
}

func responseRequestID(resp *http.Response) string {
	for _, header := range []string{"X-Request-ID", "Request-ID", "X-Correlation-ID"} {
		if value := strings.TrimSpace(resp.Header.Get(header)); value != "" {
			return value
		}
	}
	return ""
}

func transportErrorKind(err error) string {
	switch {
	case err == nil:
		return ""
	case os.IsTimeout(err):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	default:
		return "network"
	}
}

func (c *Client) GetJSON(ctx context.Context, path string, out any) error {
	return c.GetJSONWithQuery(ctx, path, nil, out)
}

func (c *Client) GetJSONWithQuery(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := c.endpoint(path)
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := decodeResponse(resp, out); err != nil {
		return err
	}
	c.notifySuccess(req)
	return nil
}

func (c *Client) GetProductJSON(ctx context.Context, path string, out any) error {
	return c.GetJSONWithQuery(ctx, path, nil, out)
}

func (c *Client) GetProductJSONWithQuery(ctx context.Context, path string, query url.Values, out any) error {
	return c.GetJSONWithQuery(ctx, path, query, out)
}

func (c *Client) PostJSON(ctx context.Context, path string, in any, out any) error {
	return c.postJSON(ctx, c.endpoint(path), in, out)
}

func (c *Client) PostProductJSON(ctx context.Context, path string, in any, out any) error {
	return c.PostJSON(ctx, path, in, out)
}

func (c *Client) PostProductJSONWithQuery(ctx context.Context, path string, query url.Values, in any, out any) error {
	endpoint := c.endpoint(path)
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	return c.postJSON(ctx, endpoint, in, out)
}

func (c *Client) postJSON(ctx context.Context, endpoint string, in any, out any) error {
	var body io.Reader
	if in != nil {
		payload, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode request JSON: %w", err)
		}
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := decodeResponse(resp, out); err != nil {
		return err
	}
	c.notifySuccess(req)
	return nil
}

type BinaryResponse struct {
	ContentType        string
	ContentDisposition string
	Body               []byte
}

func (c *Client) GetBinary(ctx context.Context, path string) (*BinaryResponse, error) {
	return c.GetBinaryWithQuery(ctx, path, nil)
}

func (c *Client) GetBinaryWithQuery(ctx context.Context, path string, query url.Values) (*BinaryResponse, error) {
	endpoint := c.endpoint(path)
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, RequestError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	c.notifySuccess(req)
	return &BinaryResponse{
		ContentType:        resp.Header.Get("Content-Type"),
		ContentDisposition: resp.Header.Get("Content-Disposition"),
		Body:               body,
	}, nil
}

func (c *Client) PostMultipartFile(ctx context.Context, path string, fieldValues map[string]string, fileField string, filePath string, out any) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fieldValues {
		if err := writer.WriteField(key, value); err != nil {
			return fmt.Errorf("write form field %s: %w", key, err)
		}
	}
	part, err := writer.CreateFormFile(fileField, filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copy form file content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(path), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := decodeResponse(resp, out); err != nil {
		return err
	}
	c.notifySuccess(req)
	return nil
}

func decodeResponse(resp *http.Response, out any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return RequestError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode response JSON: %w", err)
	}
	return nil
}
