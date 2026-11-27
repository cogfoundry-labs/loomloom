package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDirectInputAssetUploadSeparatesCredentialsAndRetriesCompletion(t *testing.T) {
	data := []byte("123456789")
	path := filepath.Join(t.TempDir(), "input.txt")
	if e := os.WriteFile(path, data, 0600); e != nil {
		t.Fatal(e)
	}
	putCount, completeCount, initCount := 0, 0, 0
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		putCount++
		if r.Method != "PUT" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("platform credential or wrong method reached OSS")
		}
		if r.Header.Get("Content-MD5") != "JfnnlDI7RTiF9RgfG2JNCw==" || r.Header.Get("x-oss-forbid-overwrite") != "true" || r.Header.Get("x-oss-object-acl") != "private" || r.ContentLength != 9 {
			t.Error("missing upload restrictions")
		}
		body, _ := io.ReadAll(r.Body)
		if !bytes.Equal(data, body) {
			t.Error("wrong bytes")
		}
		w.Header().Set("x-oss-hash-crc64ecma", "11051210869376104954")
	}))
	defer storage.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer platform-token" {
			t.Error("missing platform auth")
		}
		switch r.URL.Path {
		case "/loom/v1/inputAssets:initDirectUpload":
			initCount++
			var m map[string]any
			if e := json.NewDecoder(r.Body).Decode(&m); e != nil {
				t.Error(e)
			}
			if m["md5"] != "25f9e794323b453885f5181f1b624d0b" || m["crc64"] != "11051210869376104954" || m["sizeBytes"] != float64(9) {
				t.Errorf("wrong metadata: %+v", m)
			}
			_ = json.NewEncoder(w).Encode(directUploadAuthorization{Ticket: "same-ticket", UploadURL: storage.URL + "/fixed-object?signature=private", Headers: map[string]string{"Content-MD5": "JfnnlDI7RTiF9RgfG2JNCw==", "Content-Length": "9", "Content-Type": "text/plain", "x-oss-object-acl": "private", "x-oss-forbid-overwrite": "true"}, UploadExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), CompleteExpiresAt: time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)})
		case "/loom/v1/inputAssets:completeDirectUpload":
			completeCount++
			var m map[string]string
			_ = json.NewDecoder(r.Body).Decode(&m)
			if m["ticket"] != "same-ticket" {
				t.Error("ticket changed")
			}
			if completeCount == 1 {
				w.WriteHeader(503)
				_, _ = w.Write([]byte(`{"error":"temporary"}`))
				return
			}
			_, _ = w.Write([]byte(`{"inputAssetId":"ia_result","sizeBytes":9,"uploadedAt":123,"expiresAt":null}`))
		default:
			t.Errorf("unexpected API path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer api.Close()
	var logs bytes.Buffer
	c, e := New(Config{BaseURL: api.URL + "/loom/v1", Token: "platform-token", Timeout: time.Second, Verbose: true, LogWriter: &logs})
	if e != nil {
		t.Fatal(e)
	}
	c.httpClient.Transport = storage.Client().Transport
	var result map[string]any
	if e = c.UploadInputAsset(context.Background(), path, StreamUploadOptions{Filename: "input.txt", ContentType: "text/plain"}, &result); e != nil {
		t.Fatal(e)
	}
	if initCount != 1 || putCount != 1 || completeCount != 2 || result["inputAssetId"] != "ia_result" {
		t.Fatalf("unexpected counts %d/%d/%d or result %+v", initCount, putCount, completeCount, result)
	}
	for _, secret := range []string{"signature=private", "same-ticket", "platform-token"} {
		if bytes.Contains(logs.Bytes(), []byte(secret)) {
			t.Fatalf("sensitive value leaked in logs")
		}
	}
}
