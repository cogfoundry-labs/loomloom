package cmd

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

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
