package skill

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPackageAtomicallyInstallsAndUpdatesSameName(t *testing.T) {
	temp := t.TempDir()
	root := filepath.Join(temp, "skills")
	archive := testSkillArchive(t, "sample-skill", "# Version 1")

	result, err := InstallPackage(PackageInstallOptions{SkillRoot: root, Archive: archive})
	if err != nil {
		t.Fatalf("InstallPackage() error = %v", err)
	}
	if !result.Installed || result.Updated || result.Dir != filepath.Join(root, "sample-skill") {
		t.Fatalf("unexpected install result: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(result.Dir, "SKILL.md")); err != nil {
		t.Fatalf("installed SKILL.md missing: %v", err)
	}

	updated, err := InstallPackage(PackageInstallOptions{SkillRoot: root, Archive: testSkillArchive(t, "sample-skill", "# Version 2")})
	if err != nil {
		t.Fatalf("update InstallPackage() error = %v", err)
	}
	if !updated.Installed || !updated.Updated || updated.Dir != result.Dir {
		t.Fatalf("unexpected update result: %#v", updated)
	}
	contents, err := os.ReadFile(filepath.Join(updated.Dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read updated SKILL.md: %v", err)
	}
	if !bytes.Contains(contents, []byte("# Version 2")) {
		t.Fatalf("updated skill has old contents: %s", contents)
	}
}

func TestInstallPackageInstallsIndependentlyIntoDifferentSkillRoots(t *testing.T) {
	temp := t.TempDir()
	archive := testSkillArchive(t, "sample-skill", "# Sample")
	firstRoot := filepath.Join(temp, "codex-skills")
	secondRoot := filepath.Join(temp, "workbuddy-skills")

	first, err := InstallPackage(PackageInstallOptions{SkillRoot: firstRoot, Archive: archive})
	if err != nil {
		t.Fatalf("install first root: %v", err)
	}
	second, err := InstallPackage(PackageInstallOptions{SkillRoot: secondRoot, Archive: archive})
	if err != nil {
		t.Fatalf("install second root: %v", err)
	}
	if first.Dir == second.Dir {
		t.Fatalf("different Skill roots produced the same directory: %s", first.Dir)
	}
	for _, dir := range []string{first.Dir, second.Dir} {
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
			t.Fatalf("installed SKILL.md missing from %s: %v", dir, err)
		}
	}
}

func TestInstallPackageRejectsUnsafeArchiveWithoutReplacingExistingSkill(t *testing.T) {
	temp := t.TempDir()
	root := filepath.Join(temp, "skills")
	original := testSkillArchive(t, "sample-skill", "# Original")
	if _, err := InstallPackage(PackageInstallOptions{SkillRoot: root, Archive: original}); err != nil {
		t.Fatalf("install original: %v", err)
	}
	unsafe := testArchive(t, map[string]string{
		"sample-skill/SKILL.md": "---\nname: sample-skill\n---\n# Changed",
		"../outside.txt":        "unsafe",
	})
	if _, err := InstallPackage(PackageInstallOptions{SkillRoot: root, Archive: unsafe}); err == nil {
		t.Fatal("InstallPackage() succeeded for unsafe archive")
	}
	contents, err := os.ReadFile(filepath.Join(root, "sample-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("read original skill: %v", err)
	}
	if !bytes.Contains(contents, []byte("# Original")) {
		t.Fatalf("unsafe install replaced original skill: %s", contents)
	}
}

func TestInstalledPackageHashSkipsUnchangedArchiveAndDetectsUpdate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	archive := testSkillArchive(t, "sample-skill", "# Version 1")
	hash := archiveHash(archive)
	installed, err := InstallPackage(PackageInstallOptions{
		SkillRoot: root, SourceRef: "market:listing-1", ArchiveHash: hash, Archive: archive,
	})
	if err != nil {
		t.Fatalf("InstallPackage() error = %v", err)
	}
	markerContents, err := os.ReadFile(filepath.Join(installed.Dir, packageMarkerName))
	if err != nil {
		t.Fatalf("read package marker: %v", err)
	}
	var marker packageMarker
	if err := json.Unmarshal(markerContents, &marker); err != nil {
		t.Fatalf("package marker is not JSON: %v", err)
	}
	if marker.SchemaVersion != 1 || marker.Source != "market:listing-1" || marker.ArchiveHash != hash {
		t.Fatalf("unexpected package marker: %#v", marker)
	}
	unchanged, ok, err := FindInstalledPackage(root, "market:listing-1", hash)
	if err != nil {
		t.Fatalf("FindInstalledPackage() error = %v", err)
	}
	if !ok || !unchanged.Unchanged || unchanged.Dir != installed.Dir || unchanged.ArchiveHash != hash {
		t.Fatalf("unexpected unchanged result: ok=%t result=%#v", ok, unchanged)
	}
	if _, ok, err := FindInstalledPackage(root, "market:listing-1", "sha256:"+strings.Repeat("a", 64)); err != nil || ok {
		t.Fatalf("changed hash should require download: ok=%t err=%v", ok, err)
	}
}

func TestInstallPackageRejectsSameNameTargetOwnedByAnotherSource(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	archive := testSkillArchive(t, "sample-skill", "# First")
	if _, err := InstallPackage(PackageInstallOptions{SkillRoot: root, SourceRef: "market:listing-1", ArchiveHash: archiveHash(archive), Archive: archive}); err != nil {
		t.Fatalf("install first package: %v", err)
	}
	second := testSkillArchive(t, "sample-skill", "# Second")
	if _, err := InstallPackage(PackageInstallOptions{SkillRoot: root, SourceRef: "market:listing-2", ArchiveHash: archiveHash(second), Archive: second}); err == nil || !strings.Contains(err.Error(), "skill name conflict") {
		t.Fatalf("error=%v want skill name conflict", err)
	}
	contents, err := os.ReadFile(filepath.Join(root, "sample-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("read original skill: %v", err)
	}
	if !bytes.Contains(contents, []byte("# First")) {
		t.Fatalf("conflicting install replaced original skill: %s", contents)
	}
}

func TestInstallPackageUpdatesSameSourceWhenArchiveHashChanges(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	first := testSkillArchive(t, "sample-skill", "# First")
	if _, err := InstallPackage(PackageInstallOptions{SkillRoot: root, SourceRef: "market:listing-1", ArchiveHash: archiveHash(first), Archive: first}); err != nil {
		t.Fatalf("install first package: %v", err)
	}
	second := testSkillArchive(t, "sample-skill", "# Second")
	updated, err := InstallPackage(PackageInstallOptions{SkillRoot: root, SourceRef: "market:listing-1", ArchiveHash: archiveHash(second), Archive: second})
	if err != nil {
		t.Fatalf("update same source: %v", err)
	}
	if !updated.Updated {
		t.Fatalf("update result=%#v want Updated=true", updated)
	}
	contents, err := os.ReadFile(filepath.Join(root, "sample-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("read updated skill: %v", err)
	}
	if !bytes.Contains(contents, []byte("# Second")) {
		t.Fatalf("same-source update retained old content: %s", contents)
	}
}

func TestInstallPackageRejectsHashMismatchWithoutReplacingExistingSkill(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	first := testSkillArchive(t, "sample-skill", "# First")
	if _, err := InstallPackage(PackageInstallOptions{SkillRoot: root, SourceRef: "market:listing-1", ArchiveHash: archiveHash(first), Archive: first}); err != nil {
		t.Fatalf("install first package: %v", err)
	}
	second := testSkillArchive(t, "sample-skill", "# Second")
	if _, err := InstallPackage(PackageInstallOptions{SkillRoot: root, SourceRef: "market:listing-1", ArchiveHash: archiveHash(first), Archive: second}); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("error=%v want archive hash mismatch", err)
	}
	contents, err := os.ReadFile(filepath.Join(root, "sample-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("read original skill: %v", err)
	}
	if !bytes.Contains(contents, []byte("# First")) {
		t.Fatalf("hash mismatch replaced original skill: %s", contents)
	}
}

func TestInstallPackageRejectsUnmanagedSameNameTarget(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(root, "sample-skill")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("manual skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	archive := testSkillArchive(t, "sample-skill", "# Package")
	if _, err := InstallPackage(PackageInstallOptions{SkillRoot: root, SourceRef: "market:listing-1", ArchiveHash: archiveHash(archive), Archive: archive}); err == nil || !strings.Contains(err.Error(), "skill name conflict") {
		t.Fatalf("error=%v want skill name conflict", err)
	}
}

func TestInstallPackageRejectsReservedMarkerInZIP(t *testing.T) {
	archive := testArchive(t, map[string]string{
		"sample-skill/SKILL.md":             "---\nname: sample-skill\n---\n# Test",
		"sample-skill/" + packageMarkerName: "{}",
	})
	if _, err := InstallPackage(PackageInstallOptions{SkillRoot: t.TempDir(), SourceRef: "market:listing-1", ArchiveHash: archiveHash(archive), Archive: archive}); err == nil || !strings.Contains(err.Error(), "reserved file") {
		t.Fatalf("error=%v want reserved marker rejection", err)
	}
}

func testSkillArchive(t *testing.T, name, body string) []byte {
	t.Helper()
	return testArchive(t, map[string]string{name + "/SKILL.md": "---\nname: " + name + "\n---\n" + body})
}

func testArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, contents := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}
