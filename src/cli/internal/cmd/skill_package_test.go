package cmd

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/client"
	"github.com/cogfoundry-labs/loomloom/src/cli/internal/skill"
	"github.com/spf13/cobra"
)

func TestPackageCASQueryRequiresExpectedTupleTogether(t *testing.T) {
	tests := []struct {
		name         string
		archiveHash  string
		validationID string
		wantErr      bool
	}{
		{name: "neither"},
		{name: "both", archiveHash: "sha256:abc", validationID: "validation-1"},
		{name: "hash only", archiveHash: "sha256:abc", wantErr: true},
		{name: "validation only", validationID: "validation-1", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query, err := packageCASQuery(test.archiveHash, test.validationID)
			if (err != nil) != test.wantErr {
				t.Fatalf("packageCASQuery() error = %v, wantErr %t", err, test.wantErr)
			}
			if test.wantErr {
				return
			}
			if test.archiveHash == "" && len(query) != 0 {
				t.Fatalf("empty tuple produced query: %v", query)
			}
			if test.archiveHash != "" && (query.Get("expectedArchiveHash") != test.archiveHash || query.Get("expectedValidationId") != test.validationID) {
				t.Fatalf("unexpected query: %v", query)
			}
		})
	}
}

type packageBinaryClient struct {
	response *client.BinaryResponse
	calls    int
}

func (c *packageBinaryClient) GetBinary(context.Context, string) (*client.BinaryResponse, error) {
	c.calls++
	return c.response, nil
}

func TestPackageInstallSkipsDownloadWhenInstalledArchiveHashMatches(t *testing.T) {
	root := t.TempDir()
	archive := commandTestSkillArchive(t, "sample-skill")
	hash := commandTestArchiveHash(archive)
	if _, err := skill.InstallPackage(skill.PackageInstallOptions{
		SkillRoot: root, SourceRef: "market:listing-1", ArchiveHash: hash, Archive: archive,
	}); err != nil {
		t.Fatalf("install fixture: %v", err)
	}
	fake := &packageBinaryClient{response: &client.BinaryResponse{Body: archive}}
	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)
	if err := downloadAndInstallSkillPackage(cmd, context.Background(), fake, skillPackageSummary{
		Available: true, ArchiveHash: hash,
	}, "/archive", root, "market:listing-1"); err != nil {
		t.Fatalf("downloadAndInstallSkillPackage() error = %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("unchanged package downloaded %d times", fake.calls)
	}
	if !bytes.Contains(output.Bytes(), []byte("unchanged\ttrue")) {
		t.Fatalf("unchanged result missing from output: %s", output.String())
	}
}

func TestPackageInstallDoesNotDownloadWhenSummaryIsUnavailable(t *testing.T) {
	root := t.TempDir()
	archive := commandTestSkillArchive(t, "sample-skill")
	fake := &packageBinaryClient{response: &client.BinaryResponse{Body: archive}}
	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)
	if err := downloadAndInstallSkillPackage(cmd, context.Background(), fake, skillPackageSummary{
		Available: false, UnavailableReason: "no_published_package",
	}, "/archive", root, "official:sample"); err != nil {
		t.Fatalf("downloadAndInstallSkillPackage() error = %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("unavailable package downloaded %d times", fake.calls)
	}
	if !bytes.Contains(output.Bytes(), []byte("available\tfalse")) || !bytes.Contains(output.Bytes(), []byte("no_published_package")) {
		t.Fatalf("unavailable output missing backend summary: %s", output.String())
	}
}

func TestPackageInstallRejectsAvailableSummaryWithoutArchiveHash(t *testing.T) {
	fake := &packageBinaryClient{response: &client.BinaryResponse{Body: commandTestSkillArchive(t, "sample-skill")}}
	cmd := &cobra.Command{}
	err := downloadAndInstallSkillPackage(cmd, context.Background(), fake, skillPackageSummary{Available: true}, "/archive", t.TempDir(), "market:listing-1")
	if err == nil || !strings.Contains(err.Error(), "missing archiveHash") {
		t.Fatalf("error=%v want missing archiveHash", err)
	}
	if fake.calls != 0 {
		t.Fatalf("missing-hash package downloaded %d times", fake.calls)
	}
}

func TestMarketPackageInstallGeneratesAndVerifiesArchive(t *testing.T) {
	archive := commandTestSkillArchive(t, "generated-skill")
	hash := commandTestArchiveHash(archive)
	const summaryPath = "/loom/v1/marketListings/listing-1/skillPackage"
	for _, tc := range []struct {
		name           string
		downloadStatus int
		refreshStatus  int
		refreshed      skillPackageSummary
		wantErr        string
	}{
		{name: "generated", refreshed: skillPackageSummary{Available: true, ArchiveHash: hash}},
		{name: "generation fails", downloadStatus: http.StatusServiceUnavailable, wantErr: "status=503"},
		{name: "refresh fails", refreshStatus: http.StatusForbidden, wantErr: "status=403"},
		{name: "removed during download", refreshed: skillPackageSummary{UnavailableReason: "package_removed"}, wantErr: "unavailable after download"},
		{name: "missing hash", refreshed: skillPackageSummary{Available: true}, wantErr: "missing archiveHash"},
		{name: "publication changed", refreshed: skillPackageSummary{Available: true, ArchiveHash: commandTestArchiveHash([]byte("different"))}, wantErr: "hash mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			// Keep a previous installation to verify that any failure preserves it.
			previous := commandTestSkillArchive(t, "previous-skill")
			installed, err := skill.InstallPackage(skill.PackageInstallOptions{
				SkillRoot: root, SourceRef: "market:listing-1", ArchiveHash: commandTestArchiveHash(previous), Archive: previous,
			})
			if err != nil {
				t.Fatal(err)
			}
			markerPath := filepath.Join(installed.Dir, ".loomloom-package.json")
			before, err := os.ReadFile(markerPath)
			if err != nil {
				t.Fatal(err)
			}
			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.Path)
				switch len(requests) {
				case 1:
					_ = json.NewEncoder(w).Encode(skillPackageSummary{UnavailableReason: "no_published_package"})
				case 2:
					if tc.downloadStatus != 0 {
						w.WriteHeader(tc.downloadStatus)
						return
					}
					w.Header().Set("Content-Type", "application/zip")
					_, _ = w.Write(archive)
				case 3:
					if tc.refreshStatus != 0 {
						w.WriteHeader(tc.refreshStatus)
						return
					}
					_ = json.NewEncoder(w).Encode(tc.refreshed)
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer server.Close()
			cmd := newSkillPackageInstallMarketCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs([]string{"listing-1", "--skill-root", root})
			err = cmd.Execute()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(output.String(), "installed\ttrue") {
					t.Fatalf("expected installation result: %s", output.String())
				}
				if _, err := os.Stat(filepath.Join(root, "generated-skill", "SKILL.md")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
				if _, err := os.Stat(filepath.Join(root, "generated-skill")); !os.IsNotExist(err) {
					t.Fatalf("failed download installed a package: %v", err)
				}
				after, err := os.ReadFile(markerPath)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("previous installation changed: %v", err)
				}
			}
			wantRequests := []string{"GET " + summaryPath, "GET " + summaryPath + "/archive"}
			if tc.downloadStatus == 0 {
				wantRequests = append(wantRequests, "GET "+summaryPath)
			}
			if !reflect.DeepEqual(requests, wantRequests) {
				t.Fatalf("requests = %v, want %v", requests, wantRequests)
			}
		})
	}
}

func TestMarketPackageInstallSkipsOtherUnavailableStates(t *testing.T) {
	for _, reason := range []string{"listing_not_listed", "package_removed", "distribution_blocked", "distribution_unavailable", "unknown_reason", ""} {
		t.Run(reason, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				_ = json.NewEncoder(w).Encode(skillPackageSummary{UnavailableReason: reason})
			}))
			defer server.Close()
			cmd := newSkillPackageInstallMarketCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second})
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetArgs([]string{"listing-1", "--skill-root", t.TempDir()})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || !strings.Contains(output.String(), "available\tfalse") {
				t.Fatalf("calls = %d, output = %s", calls, output.String())
			}
		})
	}
}

func commandTestSkillArchive(t *testing.T, name string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	part, err := writer.Create(name + "/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("---\nname: " + name + "\n---\n# Test\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func commandTestArchiveHash(archive []byte) string {
	sum := sha256.Sum256(archive)
	return "sha256:" + hex.EncodeToString(sum[:])
}
