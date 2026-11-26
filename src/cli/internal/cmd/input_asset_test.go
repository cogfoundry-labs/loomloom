package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInputAssetUploadUsesStreamEndpoint(t *testing.T) {
	payload := []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom-sample-video-bytes")
	sum := sha256.Sum256(payload)
	wantSHA := hex.EncodeToString(sum[:])

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "产品实拍 01.mp4")
	if err := os.WriteFile(videoPath, payload, 0o600); err != nil {
		t.Fatalf("write temp video: %v", err)
	}

	var gotPath, gotContentType, gotFilename, gotSHA string
	var gotContentLength int64
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotFilename = r.Header.Get("X-Filename")
		gotSHA = r.Header.Get("X-Content-SHA256")
		gotContentLength = r.ContentLength
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"inputAssetId": "ia-video-1",
			"filename":     "产品实拍 01.mp4",
			"mimeType":     "video/mp4",
			"sizeBytes":    len(payload),
			"uploadedAt":   1700000000,
		})
	}))
	defer server.Close()

	var stdout bytes.Buffer
	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		token:   "test-token",
		timeout: time.Second,
		output:  "json",
	}
	cmd := newInputAssetUploadCmd(opts)
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{videoPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if gotPath != "/loom/v1/inputAssets:uploadStream" {
		t.Fatalf("path=%q want /loom/v1/inputAssets:uploadStream", gotPath)
	}
	if gotContentType != "video/mp4" {
		t.Fatalf("Content-Type=%q want video/mp4", gotContentType)
	}
	unescaped, err := url.PathUnescape(gotFilename)
	if err != nil || unescaped != "产品实拍 01.mp4" {
		t.Fatalf("X-Filename=%q unescaped=%q err=%v", gotFilename, unescaped, err)
	}
	if gotSHA != wantSHA {
		t.Fatalf("X-Content-SHA256=%q want %q", gotSHA, wantSHA)
	}
	if gotContentLength != int64(len(payload)) {
		t.Fatalf("Content-Length=%d want %d", gotContentLength, len(payload))
	}
	if !bytes.Equal(gotBody, payload) {
		t.Fatalf("uploaded bytes mismatch: got %d bytes, want %d", len(gotBody), len(payload))
	}

	var resp uploadInputAssetResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("decode stdout JSON: %v (raw=%q)", err, stdout.String())
	}
	if resp.InputAssetID != "ia-video-1" || resp.MimeType != "video/mp4" || int64(resp.SizeBytes) != int64(len(payload)) {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestInputAssetUploadContentTypeOverrideAndFallback(t *testing.T) {
	dir := t.TempDir()
	unknownPath := filepath.Join(dir, "blob.custombin")
	if err := os.WriteFile(unknownPath, []byte{0x00, 0x01, 0x02, 0x03}, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got, err := detectInputAssetContentType(unknownPath, "")
	if err != nil {
		t.Fatalf("detectInputAssetContentType failed: %v", err)
	}
	if got != "application/octet-stream" {
		t.Fatalf("fallback Content-Type=%q want application/octet-stream", got)
	}

	gotOverride, err := detectInputAssetContentType(unknownPath, "text/plain; charset=utf-8")
	if err != nil {
		t.Fatalf("detectInputAssetContentType override failed: %v", err)
	}
	if gotOverride != "text/plain" {
		t.Fatalf("override Content-Type=%q want text/plain", gotOverride)
	}
}

func TestResolveInputAssetUploadTimeout(t *testing.T) {
	root := NewRootCmd()
	uploadCmd, _, err := root.Find([]string{"input-asset", "upload"})
	if err != nil {
		t.Fatalf("find input-asset upload: %v", err)
	}

	if got := resolveInputAssetUploadTimeout(uploadCmd, defaultHTTPTimeout); got != defaultInputAssetUploadTimeout {
		t.Fatalf("default timeout=%v want %v", got, defaultInputAssetUploadTimeout)
	}

	if err := root.PersistentFlags().Set("timeout", "30s"); err != nil {
		t.Fatalf("set --timeout flag: %v", err)
	}
	if got := resolveInputAssetUploadTimeout(uploadCmd, defaultHTTPTimeout); got != defaultHTTPTimeout {
		t.Fatalf("explicit --timeout=30s got %v want %v", got, defaultHTTPTimeout)
	}
}

func TestInputAssetUploadTextOutput(t *testing.T) {
	dir := t.TempDir()
	txtPath := filepath.Join(dir, "brief.txt")
	if err := os.WriteFile(txtPath, []byte("hello world"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loom/v1/inputAssets:uploadStream" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "text/plain" {
			t.Fatalf("Content-Type=%q want text/plain", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"inputAssetId":"ia-txt-1","filename":"brief.txt","mimeType":"text/plain","sizeBytes":11,"uploadedAt":1700000000}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	opts := &rootOptions{
		server:  server.URL + "/loom/v1",
		token:   "test-token",
		timeout: time.Second,
		output:  "text",
	}
	cmd := newInputAssetUploadCmd(opts)
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{txtPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"input_asset_id\tia-txt-1", "filename\tbrief.txt", "mime_type\ttext/plain", "size_bytes\t11"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output=%q missing %q", out, want)
		}
	}
}
