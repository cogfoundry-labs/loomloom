package skill

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrOutputDirNotEmpty    = errors.New("output directory is not empty")
	ErrOutputPathNotDir     = errors.New("output path exists and is not a directory")
	ErrOutputDirUnavailable = errors.New("output directory is unavailable")
)

var allowedSkillFiles = map[string]struct{}{
	"SKILL.md":            {},
	"loomloom-skill.json": {},
}

func checkOutputDir(outputDir string, dryRun bool) error {
	info, err := os.Stat(outputDir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%w: %s", ErrOutputPathNotDir, outputDir)
		}
		entries, err := os.ReadDir(outputDir)
		if err != nil {
			return fmt.Errorf("%w: read output directory: %w", ErrOutputDirUnavailable, err)
		}
		if len(entries) > 0 {
			return fmt.Errorf("%w: %s", ErrOutputDirNotEmpty, outputDir)
		}
		if dryRun {
			return probeWritable(filepath.Dir(outputDir))
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("%w: stat output directory: %w", ErrOutputDirUnavailable, err)
	}
	parent := filepath.Dir(outputDir)
	parentInfo, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("%w: output parent directory is not available: %w", ErrOutputDirUnavailable, err)
	}
	if !parentInfo.IsDir() {
		return fmt.Errorf("%w: output parent is not a directory: %s", ErrOutputDirUnavailable, parent)
	}
	return probeWritable(parent)
}

func writeSkillDir(outputDir string, skillMarkdown string, metadataJSON []byte) error {
	parent := filepath.Dir(outputDir)
	tmpDir, err := os.MkdirTemp(parent, "."+safeTempPrefix(filepath.Base(outputDir))+"-")
	if err != nil {
		return fmt.Errorf("create temporary skill directory: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	if err := os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte(skillMarkdown), 0o644); err != nil {
		return fmt.Errorf("write temporary SKILL.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "loomloom-skill.json"), metadataJSON, 0o644); err != nil {
		return fmt.Errorf("write temporary loomloom-skill.json: %w", err)
	}
	bestEffortSyncDir(tmpDir)
	if err := removeEmptyOutputDirIfPresent(outputDir); err != nil {
		return err
	}
	if err := os.Rename(tmpDir, outputDir); err != nil {
		return fmt.Errorf("publish skill directory: %w", err)
	}
	cleanup = false
	bestEffortSyncDir(parent)
	return nil
}

func removeEmptyOutputDirIfPresent(outputDir string) error {
	info, err := os.Stat(outputDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: stat output directory before publish: %w", ErrOutputDirUnavailable, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrOutputPathNotDir, outputDir)
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("%w: read output directory before publish: %w", ErrOutputDirUnavailable, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("%w: %s", ErrOutputDirNotEmpty, outputDir)
	}
	if err := os.Remove(outputDir); err != nil {
		return fmt.Errorf("%w: remove empty output directory before publish: %w", ErrOutputDirUnavailable, err)
	}
	return nil
}

func probeWritable(dir string) error {
	tmpDir, err := os.MkdirTemp(dir, ".loomloom-skill-probe-")
	if err != nil {
		return fmt.Errorf("%w: probe write in %s: %w", ErrOutputDirUnavailable, dir, err)
	}
	if err := os.RemoveAll(tmpDir); err != nil {
		return fmt.Errorf("%w: remove write probe in %s: %w", ErrOutputDirUnavailable, dir, err)
	}
	return nil
}

func bestEffortSyncDir(dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	defer f.Close()
	_ = f.Sync()
}

func safeTempPrefix(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "loomloom-skill"
	}
	replacer := strings.NewReplacer(string(filepath.Separator), "-", ":", "-", " ", "-")
	return replacer.Replace(name)
}

func Uninstall(opts UninstallOptions) (*UninstallResult, error) {
	preview := previewUninstall(opts)
	if opts.DryRun {
		return &UninstallResult{UninstallPreview: preview}, nil
	}
	if !preview.Removable {
		return nil, fmt.Errorf("%s: %s", preview.BlockingReason, firstIssueMessage(preview.Errors))
	}
	if err := os.RemoveAll(preview.Dir); err != nil {
		return nil, fmt.Errorf("remove skill directory: %w", err)
	}
	preview.WillDelete = nil
	return &UninstallResult{
		UninstallPreview: preview,
		Uninstalled:      true,
	}, nil
}

func previewUninstall(opts UninstallOptions) UninstallPreview {
	dir := strings.TrimSpace(opts.Dir)
	preview := UninstallPreview{
		PreviewSchemaVersion: UninstallPreviewSchemaVersion,
		Removable:            true,
		Dir:                  dir,
		Warnings:             []Issue{},
		Errors:               []Issue{},
	}
	block := func(code string, message string, action string) UninstallPreview {
		preview.Removable = false
		preview.BlockingReason = code
		preview.RecommendedAction = action
		preview.Errors = append(preview.Errors, Issue{Code: code, Message: message})
		return preview
	}

	if dir == "" {
		return block("dir_required", "--dir is required", "Pass the directory of one LoomLoom-generated skill.")
	}
	linkInfo, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return block("dir_not_found", fmt.Sprintf("skill directory does not exist: %s", dir), "Pass an existing LoomLoom-generated skill directory.")
		}
		return block("dir_unavailable", fmt.Sprintf("stat skill directory: %v", err), "Check the directory path and permissions.")
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return block("dir_is_symlink", fmt.Sprintf("skill path is a symlink: %s", dir), "Pass the real LoomLoom-generated skill directory, not a symlink.")
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return block("dir_not_found", fmt.Sprintf("skill directory does not exist: %s", dir), "Pass an existing LoomLoom-generated skill directory.")
		}
		return block("dir_unavailable", fmt.Sprintf("stat skill directory: %v", err), "Check the directory path and permissions.")
	}
	if !info.IsDir() {
		return block("dir_not_directory", fmt.Sprintf("skill path is not a directory: %s", dir), "Pass the directory of one LoomLoom-generated skill.")
	}

	metadataPath := filepath.Join(dir, "loomloom-skill.json")
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return block("metadata_missing", "loomloom-skill.json was not found", "Only LoomLoom-generated skill directories can be uninstalled by this command.")
		}
		return block("metadata_unavailable", fmt.Sprintf("read loomloom-skill.json: %v", err), "Check the metadata file and permissions.")
	}
	var metadata Metadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return block("metadata_invalid", fmt.Sprintf("parse loomloom-skill.json: %v", err), "Only valid LoomLoom skill metadata can be uninstalled by this command.")
	}
	preview.SkillName = metadata.SkillName
	preview.DisplayName = metadata.DisplayName
	preview.Agent = metadata.Agent
	preview.SourceType = metadata.SourceType
	preview.SourceID = metadata.SourceID
	if metadata.SchemaVersion != MetadataSchemaVersion {
		return block("metadata_schema_mismatch", fmt.Sprintf("metadata schema_version %q is not %q", metadata.SchemaVersion, MetadataSchemaVersion), "Only LoomLoom-generated skill directories can be uninstalled by this command.")
	}
	if metadata.GeneratedBy != "loomloom-cli" {
		return block("metadata_generator_mismatch", fmt.Sprintf("metadata generated_by %q is not loomloom-cli", metadata.GeneratedBy), "Only LoomLoom CLI generated skills can be uninstalled by this command.")
	}
	if strings.TrimSpace(metadata.SkillName) == "" {
		return block("metadata_skill_name_missing", "metadata skill_name is empty", "Check the metadata file before uninstalling.")
	}
	if strings.TrimSpace(metadata.Agent) == "" {
		return block("metadata_agent_missing", "metadata agent is empty", "Check the metadata file before uninstalling.")
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		if os.IsNotExist(err) {
			return block("skill_file_missing", "SKILL.md was not found", "Only complete LoomLoom-generated skill directories can be uninstalled by this command.")
		}
		return block("skill_file_unavailable", fmt.Sprintf("stat SKILL.md: %v", err), "Check the skill file and permissions.")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return block("dir_unavailable", fmt.Sprintf("read skill directory: %v", err), "Check the directory permissions.")
	}
	unexpected := []string{}
	willDelete := make([]string, 0, len(entries)+1)
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		willDelete = append(willDelete, path)
		if _, ok := allowedSkillFiles[entry.Name()]; !ok {
			unexpected = append(unexpected, path)
		}
	}
	sort.Strings(unexpected)
	sort.Strings(willDelete)
	willDelete = append(willDelete, dir)
	preview.WillDelete = willDelete
	preview.UnexpectedFiles = unexpected
	if len(unexpected) > 0 {
		preview.Warnings = append(preview.Warnings, Issue{
			Code:    "unexpected_files",
			Message: fmt.Sprintf("skill directory contains %d unexpected file(s)", len(unexpected)),
		})
		if !opts.Force {
			return block("unexpected_files", "skill directory contains unexpected files", "Review the directory and retry with --force if you intentionally want to remove it.")
		}
	}
	return preview
}

func firstIssueMessage(issues []Issue) string {
	if len(issues) == 0 {
		return "uninstall validation failed"
	}
	return issues[0].Message
}
