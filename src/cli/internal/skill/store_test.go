package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSkillDirPublishesIntoExistingEmptyDirectory(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "skill")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := writeSkillDir(outputDir, "# Test Skill\n", []byte(`{"schema_version":"test"}`+"\n")); err != nil {
		t.Fatalf("writeSkillDir error = %v", err)
	}

	skillBytes, err := os.ReadFile(filepath.Join(outputDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	if string(skillBytes) != "# Test Skill\n" {
		t.Fatalf("SKILL.md=%q", string(skillBytes))
	}

	metadataBytes, err := os.ReadFile(filepath.Join(outputDir, "loomloom-skill.json"))
	if err != nil {
		t.Fatalf("read loomloom-skill.json: %v", err)
	}
	if string(metadataBytes) != "{\"schema_version\":\"test\"}\n" {
		t.Fatalf("metadata=%q", string(metadataBytes))
	}
}

func TestWriteSkillDirRejectsExistingNonEmptyDirectory(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "skill")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := checkOutputDir(outputDir, false)
	if err == nil {
		t.Fatal("checkOutputDir error = nil, want non-empty directory error")
	}
}

func TestUninstallBlocksMetadataSchemaMismatch(t *testing.T) {
	dir := writeStoreTestSkillDir(t, `{
		"schema_version":"other/v1",
		"generated_by":"loomloom-cli",
		"agent":"codex",
		"skill_name":"test-skill"
	}`, true)

	result, err := Uninstall(UninstallOptions{Dir: dir, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run error = %v", err)
	}
	if result.Removable {
		t.Fatal("Removable=true, want false")
	}
	if result.BlockingReason != "metadata_schema_mismatch" {
		t.Fatalf("BlockingReason=%q", result.BlockingReason)
	}
}

func TestUninstallBlocksGeneratedByMismatch(t *testing.T) {
	dir := writeStoreTestSkillDir(t, `{
		"schema_version":"loomloom-skill/v1",
		"generated_by":"someone-else",
		"agent":"codex",
		"skill_name":"test-skill"
	}`, true)

	result, err := Uninstall(UninstallOptions{Dir: dir, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run error = %v", err)
	}
	if result.Removable {
		t.Fatal("Removable=true, want false")
	}
	if result.BlockingReason != "metadata_generator_mismatch" {
		t.Fatalf("BlockingReason=%q", result.BlockingReason)
	}
}

func TestUninstallBlocksMissingSkillMarkdown(t *testing.T) {
	dir := writeStoreTestSkillDir(t, `{
		"schema_version":"loomloom-skill/v1",
		"generated_by":"loomloom-cli",
		"agent":"codex",
		"skill_name":"test-skill"
	}`, false)

	result, err := Uninstall(UninstallOptions{Dir: dir, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run error = %v", err)
	}
	if result.Removable {
		t.Fatal("Removable=true, want false")
	}
	if result.BlockingReason != "skill_file_missing" {
		t.Fatalf("BlockingReason=%q", result.BlockingReason)
	}
}

func TestUninstallBlocksSymlinkDirectory(t *testing.T) {
	targetDir := writeStoreTestSkillDir(t, `{
		"schema_version":"loomloom-skill/v1",
		"generated_by":"loomloom-cli",
		"agent":"codex",
		"skill_name":"test-skill"
	}`, true)
	linkDir := filepath.Join(t.TempDir(), "skill-link")
	if err := os.Symlink(targetDir, linkDir); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	result, err := Uninstall(UninstallOptions{Dir: linkDir, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run error = %v", err)
	}
	if result.Removable {
		t.Fatal("Removable=true, want false")
	}
	if result.BlockingReason != "dir_is_symlink" {
		t.Fatalf("BlockingReason=%q", result.BlockingReason)
	}
}

func writeStoreTestSkillDir(t *testing.T, metadata string, includeSkillMarkdown bool) string {
	t.Helper()
	dir := t.TempDir()
	if includeSkillMarkdown {
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "loomloom-skill.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
