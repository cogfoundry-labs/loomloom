package client

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc64"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type directUploadAuthorization struct {
	Ticket            string            `json:"ticket"`
	UploadURL         string            `json:"uploadUrl"`
	Headers           map[string]string `json:"headers"`
	UploadExpiresAt   string            `json:"uploadExpiresAt"`
	CompleteExpiresAt string            `json:"completeExpiresAt"`
}

// UploadInputAsset prefers direct OSS uploads. Only an unavailable init endpoint
// may fall back to the legacy stream; uncertain completions retry the same ticket.
func (c *Client) UploadInputAsset(ctx context.Context, path string, opts StreamUploadOptions, out any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open input file: %w", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		return fmt.Errorf("input file must be a nonempty regular file")
	}
	limit := int64(10 << 20)
	if strings.HasPrefix(opts.ContentType, "video/") {
		limit = 200 << 20
	}
	if info.Size() > limit {
		return fmt.Errorf("input file exceeds %d MiB limit", limit>>20)
	}
	md5sum := md5.New()
	crc := crc64.New(crc64.MakeTable(crc64.ECMA))
	n, err := io.Copy(io.MultiWriter(md5sum, crc), f)
	if err != nil {
		return err
	}
	if n != info.Size() {
		return fmt.Errorf("input file changed while hashing")
	}
	digest := md5sum.Sum(nil)
	metadata := map[string]any{"filename": opts.Filename, "mimeType": opts.ContentType, "sizeBytes": n, "md5": hex.EncodeToString(digest), "crc64": strconv.FormatUint(crc.Sum64(), 10)}
	var auth directUploadAuthorization
	if err = c.PostJSON(ctx, "/inputAssets:initDirectUpload", metadata, &auth); err != nil {
		var response RequestError
		if errors.As(err, &response) {
			var body struct {
				Code string `json:"code"`
			}
			_ = json.Unmarshal([]byte(response.Body), &body)
			if response.StatusCode == 404 || response.StatusCode == 501 || body.Code == "input_direct_upload_disabled" {
				return c.PostStreamFile(ctx, "/inputAssets:uploadStream", path, opts, out)
			}
		}
		return err
	}
	u, err := url.Parse(auth.UploadURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || auth.Ticket == "" {
		return fmt.Errorf("invalid direct upload authorization")
	}
	for _, value := range []string{auth.UploadExpiresAt, auth.CompleteExpiresAt} {
		t, e := time.Parse(time.RFC3339, value)
		if e != nil || !t.After(time.Now()) {
			return fmt.Errorf("expired direct upload authorization")
		}
	}
	want := map[string]string{"content-md5": base64.StdEncoding.EncodeToString(digest), "content-length": strconv.FormatInt(n, 10), "content-type": opts.ContentType, "x-oss-object-acl": "private", "x-oss-forbid-overwrite": "true"}
	if len(auth.Headers) != len(want) {
		return fmt.Errorf("invalid direct upload headers")
	}
	for k, v := range auth.Headers {
		expected, ok := want[strings.ToLower(k)]
		if !ok || expected != v {
			return fmt.Errorf("invalid direct upload headers")
		}
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, auth.UploadURL, io.LimitReader(f, n))
	if err != nil {
		return fmt.Errorf("invalid direct upload request")
	}
	req.ContentLength = n
	for k, v := range auth.Headers {
		req.Header.Set(k, v)
	}
	// This client has no platform authentication, cookies, redirects or URL logs.
	ossClient := &http.Client{Timeout: c.httpClient.Timeout, Transport: c.httpClient.Transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, putErr := ossClient.Do(req)
	if putErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("object upload failed; retry the upload")
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	_ = response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("object upload failed: status=%d", response.StatusCode)
	}
	if returned := response.Header.Get("x-oss-hash-crc64ecma"); returned != "" && returned != metadata["crc64"] {
		return fmt.Errorf("object upload CRC64 mismatch")
	}
	var raw json.RawMessage
	for attempt := 0; attempt < 3; attempt++ {
		err = c.PostJSON(ctx, "/inputAssets:completeDirectUpload", map[string]string{"ticket": auth.Ticket}, &raw)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var response RequestError
		if errors.As(err, &response) && response.StatusCode < 500 && response.StatusCode != 408 && response.StatusCode != 429 {
			return err
		}
	}
	if err != nil {
		return fmt.Errorf("upload completion is unconfirmed; retry uploading the file: %w", err)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields["expiresAt"] == nil {
		return fmt.Errorf("upload response is missing expiresAt")
	}
	if string(fields["expiresAt"]) != "null" {
		var value string
		if json.Unmarshal(fields["expiresAt"], &value) != nil {
			return fmt.Errorf("invalid expiresAt")
		}
		if _, err = time.Parse(time.RFC3339, value); err != nil {
			return fmt.Errorf("invalid expiresAt")
		}
	}
	return json.Unmarshal(raw, out)
}
