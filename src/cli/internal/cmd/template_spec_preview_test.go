package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTemplateSpecViewLinkUsesExactVersionWithoutPreview(t *testing.T) {
	isolateCmdConfigHome(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "GET" || r.URL.Path != "/loom/v1/users/me/templates/template-a/versions/version-b/viewLink" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Loomloom-Preview-Writer") != "" {
			t.Error("writer sent to saved-version endpoint")
		}
		_, _ = fmt.Fprint(w, `{"templateId":"template-a","versionId":"version-b","viewUrl":"https://work.example/zh/workbench/templates/template-a/versions/version-b"}`)
	}))
	defer server.Close()
	var out bytes.Buffer
	cmd := newTemplateSpecViewLinkCmd(&rootOptions{server: server.URL + "/loom/v1", token: "test-token", output: "json", timeout: time.Second})
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"template-a", "version-b"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var link templateVersionViewLink
	if err := json.Unmarshal(out.Bytes(), &link); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || link.VersionID != "version-b" || !strings.HasPrefix(link.ViewURL, "https://work.example/zh/") {
		t.Fatalf("unexpected link: %+v requests=%d", link, requests)
	}
}

func TestSavingReturnsExactViewLinkWithoutRepeatingMutations(t *testing.T) {
	for _, commandName := range []string{"create", "create-version"} {
		for _, mode := range []string{"json", "table"} {
			for _, outcome := range []string{"ok", "unavailable", "wrong-version", "unsafe-url"} {
				t.Run(commandName+"/"+mode+"/"+outcome, func(t *testing.T) {
					isolateCmdConfigHome(t)
					path := filepath.Join(t.TempDir(), "draft.json")
					content := `{"meta":{"name":"Draft"},"templateInputs":{"prompt":{"kind":"value","valueType":"string","required":true,"blankPolicy":"error"}},"steps":[{"stepId":"stp_text01","displayName":"Text","executionBinding":{"kind":"fixedModelContract","subjectRevisionId":"text-contract"},"inputBindings":{"prompt":{"source":"templateInput","inputKey":"prompt"}}}],"workbook":{}}`
					if err := os.WriteFile(path, []byte(content), 0600); err != nil {
						t.Fatal(err)
					}
					creates, saves, reads := 0, 0, 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						switch r.Method + " " + r.URL.Path {
						case "POST /loom/v1/templateSpecs:validate":
							fmt.Fprint(w, `{"valid":true}`)
						case "POST /loom/v1/users/me/templates":
							creates++
							fmt.Fprint(w, `{"templateId":"tmpl_123","name":"Draft","status":"active"}`)
						case "POST /loom/v1/users/me/templates/tmpl_123/versions":
							saves++
							fmt.Fprint(w, `{"versionId":"ver_123","versionNumber":1,"definitionHash":"hash"}`)
						case "GET /loom/v1/users/me/templates/tmpl_123/versions/ver_123/viewLink":
							reads++
							if saves != 1 {
								t.Error("link read before successful save")
							}
							switch outcome {
							case "unavailable":
								w.WriteHeader(503)
								fmt.Fprint(w, `{"error":"sensitive-error-body"}`)
							case "wrong-version":
								fmt.Fprint(w, `{"templateId":"tmpl_123","versionId":"other","viewUrl":"https://work.example/other"}`)
							case "unsafe-url":
								fmt.Fprint(w, `{"templateId":"tmpl_123","versionId":"ver_123","viewUrl":"javascript:alert(1)"}`)
							default:
								fmt.Fprint(w, `{"templateId":"tmpl_123","versionId":"ver_123","viewUrl":"https://work.example/templates/tmpl_123/versions/ver_123"}`)
							}
						default:
							t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
							w.WriteHeader(404)
						}
					}))
					defer server.Close()
					opts := &rootOptions{server: server.URL + "/loom/v1", token: "test-token", timeout: time.Second, output: mode}
					command := newTemplateSpecCreateCmd(opts)
					args := []string{path}
					if commandName == "create-version" {
						command = newTemplateSpecCreateVersionCmd(opts)
						args = []string{"tmpl_123", path}
					}
					var out, warnings bytes.Buffer
					command.SetOut(&out)
					command.SetErr(&warnings)
					command.SetArgs(args)
					if err := command.Execute(); err != nil {
						t.Fatalf("save must remain successful: %v", err)
					}
					wantCreates := 0
					if commandName == "create" {
						wantCreates = 1
					}
					if creates != wantCreates || saves != 1 || reads != 1 {
						t.Fatalf("creates=%d saves=%d reads=%d", creates, saves, reads)
					}
					if !strings.Contains(out.String(), "ver_123") {
						t.Fatal("successful version ID missing")
					}
					if mode == "json" {
						var result map[string]any
						if err := json.Unmarshal(out.Bytes(), &result); err != nil {
							t.Fatal(err)
						}
						if outcome == "ok" {
							if result["viewUrl"] != "https://work.example/templates/tmpl_123/versions/ver_123" || result["viewLinkError"] != nil {
								t.Fatalf("result=%v", result)
							}
						} else if result["viewUrl"] != nil || result["viewLinkError"] != "view_link_unavailable" {
							t.Fatalf("result=%v", result)
						}
					} else if outcome == "ok" && !strings.Contains(out.String(), "view_url\thttps://work.example/") {
						t.Fatal("text link missing")
					}
					if outcome == "ok" && warnings.Len() != 0 {
						t.Fatal(warnings.String())
					}
					if outcome != "ok" && !strings.Contains(warnings.String(), "Version saved successfully") {
						t.Fatal("missing nonfatal warning")
					}
					if strings.Contains(out.String()+warnings.String(), "sensitive-error-body") {
						t.Fatal("server body leaked")
					}
				})
			}
		}
	}
}

func TestLocalPreviewDoesNotUseCloudIdentityOrFetchViewer(t *testing.T) {
	isolateCmdConfigHome(t)
	requests := atomic.Int32{}
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(500) }))
	defer remote.Close()
	t.Setenv("LOOMLOOM_SERVER", "invalid-cloud-server")
	t.Setenv("LOOMLOOM_TOKEN", "unbound-cloud-key")
	file := filepath.Join(t.TempDir(), "draft.json")
	if err := os.WriteFile(file, []byte(`{"steps":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	command := NewRootCmd()
	command.SetContext(ctx)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"template-spec", "preview", file, "--no-open", "--output", "json", "--viewer-url", remote.URL + "/viewer/v1/bootstrap.js"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 || !strings.Contains(output.String(), `"type":"ready"`) {
		t.Fatal("local command depended on cloud or downloaded viewer resources")
	}
}

func TestLocalPreviewOpensBrowserAndKeepsRunningOnFailure(t *testing.T) {
	previous := openPreviewBrowser
	defer func() { openPreviewBrowser = previous }()
	called := false
	openPreviewBrowser = func(target string) error {
		u, err := url.Parse(target)
		if err != nil || u.Hostname() != "127.0.0.1" || !strings.HasPrefix(u.Fragment, "token=") {
			t.Fatal("unsafe browser target")
		}
		called = true
		return fmt.Errorf("browser not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	command := newTemplateSpecPreviewCmd(&rootOptions{output: "json"})
	command.SetContext(ctx)
	var output, warnings bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&warnings)
	command.SetArgs([]string{filepath.Join(t.TempDir(), "draft.json"), "--viewer-url", "https://viewer.example/viewer/v1/bootstrap.js"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called || !strings.Contains(warnings.String(), "local viewUrl above") || !strings.Contains(output.String(), `"type":"ended"`) {
		t.Fatal("browser fallback failed")
	}
}
