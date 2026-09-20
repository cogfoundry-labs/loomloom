package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// Execute through the parent commands to cover registration as well as behavior.
func packageDownloadTestCommand(opts *rootOptions, listing bool) (*cobra.Command, []string) {
	if listing {
		return newListingCmd(opts), []string{"download-skill-package", "listing-1", "--version-id", "package-version-1"}
	}
	return newSkillCmd(opts), []string{"package", "private", "download", "template-1"}
}

func TestSkillPackageDownloadsSaveZIP(t *testing.T) {
	archive := commandTestSkillArchive(t, "download-only")
	for _, listing := range []bool{false, true} {
		for _, mode := range []string{"file", "directory", "fallback", "unsafe-filename"} {
			t.Run(fmt.Sprintf("listing=%t/%s", listing, mode), func(t *testing.T) {
				root := t.TempDir()
				outputPath := filepath.Join(root, "saved.zip")
				filename := "saved.zip"
				disposition := `attachment; filename="server-package.zip"`
				if mode != "file" {
					outputPath = root
					filename = "server-package.zip"
				}
				if mode == "fallback" || mode == "unsafe-filename" {
					disposition = ""
					if mode == "unsafe-filename" {
						disposition = `attachment; filename=".."`
					}
					filename = "private-skill-package.zip"
					if listing {
						filename = "listing-skill-package.zip"
					}
				}
				target := filepath.Join(root, filename)
				if mode == "file" {
					if err := os.WriteFile(target, []byte("previous ZIP"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					wantPath := "/loom/v1/users/me/templates/template-1/skillPackage/archive"
					if listing {
						wantPath = "/loom/v1/creators/me/marketListings/listing-1/skillPackageVersions/package-version-1/archive"
					}
					if r.Method != http.MethodGet || r.URL.RequestURI() != wantPath || r.Header.Get("Authorization") != "Bearer test-token" {
						t.Errorf("unexpected download request: %s %s", r.Method, r.URL.RequestURI())
					}
					w.Header().Set("Content-Type", "application/zip")
					w.Header().Set("Content-Disposition", disposition)
					_, _ = w.Write(archive)
				}))
				defer server.Close()
				format := "json"
				if mode == "file" {
					format = "text"
				}
				cmd, args := packageDownloadTestCommand(&rootOptions{server: server.URL + "/loom/v1", token: "test-token", timeout: time.Second, output: format}, listing)
				var output bytes.Buffer
				cmd.SetOut(&output)
				cmd.SetErr(&output)
				cmd.SetArgs(append(args, "-f", outputPath))
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(target)
				if err != nil || !bytes.Equal(data, archive) {
					t.Fatalf("ZIP was not saved unchanged: %v", err)
				}
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 1 || calls != 1 {
					t.Fatalf("unexpected extraction, temporary file, or extra request: entries=%v calls=%d err=%v", entries, calls, err)
				}
				if format == "json" {
					var result skillPackageDownloadResult
					if err := json.Unmarshal(output.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if result.Path != target || result.Filename != filename || result.Size != len(archive) {
						t.Fatalf("unexpected metadata: %+v", result)
					}
					if listing && (result.ListingID != "listing-1" || result.SkillPackageVersionID != "package-version-1" || result.TemplateID != "") {
						t.Fatalf("wrong Listing identifiers: %+v", result)
					}
					if !listing && (result.TemplateID != "template-1" || result.ListingID != "" || result.SkillPackageVersionID != "") {
						t.Fatalf("wrong template identifiers: %+v", result)
					}
				} else if !strings.Contains(output.String(), "path\t"+target) || !strings.Contains(output.String(), fmt.Sprintf("size\t%d", len(archive))) {
					t.Fatalf("missing text metadata: %s", output.String())
				}
			})
		}
	}
}

func TestSkillPackageDownloadsPreserveFilesOnFailure(t *testing.T) {
	for _, listing := range []bool{false, true} {
		for _, status := range []int{401, 403, 404, 503, 200} {
			t.Run(fmt.Sprintf("listing=%t/status=%d", listing, status), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if status == http.StatusOK {
						w.Header().Set("Content-Length", "1000") // interrupted archive stream
					}
					w.WriteHeader(status)
					_, _ = w.Write([]byte("incomplete or denied"))
				}))
				defer server.Close()
				for _, existing := range []bool{false, true} {
					root := t.TempDir()
					target := filepath.Join(root, "package.zip")
					previous := []byte("existing ZIP")
					if existing {
						if err := os.WriteFile(target, previous, 0o600); err != nil {
							t.Fatal(err)
						}
					}
					cmd, args := packageDownloadTestCommand(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second}, listing)
					cmd.SetOut(new(bytes.Buffer))
					cmd.SetErr(new(bytes.Buffer))
					cmd.SetArgs(append(args, "--output-file", target))
					err := cmd.Execute()
					if err == nil || (status != 200 && !strings.Contains(err.Error(), fmt.Sprintf("status=%d", status))) {
						t.Fatalf("unexpected error: %v", err)
					}
					data, err := os.ReadFile(target)
					if existing && (err != nil || !bytes.Equal(data, previous)) {
						t.Fatalf("existing ZIP changed: %v", err)
					}
					if !existing && !os.IsNotExist(err) {
						t.Fatalf("failed download created output: %v", err)
					}
				}
			})
		}
	}
}

func TestSkillPackageDownloadRequiredArguments(t *testing.T) {
	for _, tc := range []struct {
		listing bool
		args    []string
	}{
		{args: []string{"package", "private", "download", "template-1"}},
		{args: []string{"package", "private", "download", "template-1", "-f", " "}},
		{args: []string{"package", "private", "download", " ", "-f", "out.zip"}},
		{args: []string{"package", "private", "download", "-f", "out.zip"}},
		{listing: true, args: []string{"download-skill-package", "listing-1", "-f", "out.zip"}},
		{listing: true, args: []string{"download-skill-package", "listing-1", "--version-id", "v"}},
		{listing: true, args: []string{"download-skill-package", "listing-1", "--version-id", " ", "-f", "out.zip"}},
		{listing: true, args: []string{"download-skill-package", " ", "--version-id", "v", "-f", "out.zip"}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			cmd, _ := packageDownloadTestCommand(&rootOptions{}, tc.listing)
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err == nil || strings.Contains(err.Error(), "server") {
				t.Fatalf("expected argument error before creating a client, got %v", err)
			}
		})
	}
}

func TestWriteSkillPackageArchiveCleansUpOnRenameFailure(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "existing-directory")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeSkillPackageArchive(target, []byte("archive")); err == nil {
		t.Fatal("expected rename failure")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() || entries[0].Name() != "existing-directory" {
		t.Fatalf("destination changed or temporary file leaked: %v, %v", entries, err)
	}
}
