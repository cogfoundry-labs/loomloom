package skill

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type PackageInstallOptions struct {
	SkillRoot   string
	SourceRef   string
	ArchiveHash string
	Archive     []byte
}

type PackageInstallResult struct {
	Installed   bool   `json:"installed"`
	Updated     bool   `json:"updated"`
	Unchanged   bool   `json:"unchanged"`
	SkillName   string `json:"skillName,omitempty"`
	Dir         string `json:"dir,omitempty"`
	ArchiveHash string `json:"archiveHash,omitempty"`
}

// InstallPackage verifies and atomically installs a backend-provided Skill ZIP.
// It intentionally does not assume a particular Agent: callers provide its Skill root.
func InstallPackage(opts PackageInstallOptions) (*PackageInstallResult, error) {
	root, err := absoluteSkillRoot(opts.SkillRoot)
	if err != nil {
		return nil, err
	}
	if len(opts.Archive) == 0 {
		return nil, errors.New("package archive is empty")
	}
	if len(opts.Archive) > 10<<20 {
		return nil, errors.New("package archive exceeds 10 MiB")
	}
	actualHash := archiveHash(opts.Archive)
	if expected := normalizeArchiveHash(opts.ArchiveHash); expected != "" && expected != actualHash {
		return nil, fmt.Errorf("package archive hash mismatch: expected %s, got %s", expected, actualHash)
	}

	skillName, stagedDir, cleanup, err := stagePackage(root, opts.Archive)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if err := writePackageMarker(stagedDir, opts.SourceRef, actualHash); err != nil {
		return nil, err
	}
	target := filepath.Join(root, skillName)
	if !isWithin(root, target) {
		return nil, errors.New("package target escapes skill root")
	}

	hadPrevious := false
	backup := ""
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("existing skill target must not be a symlink")
		}
		if !info.IsDir() {
			return nil, errors.New("existing skill target is not a directory")
		}
		if err := ensureTargetOwnedBySource(target, opts.SourceRef); err != nil {
			return nil, err
		}
		hadPrevious = true
		backup, err = os.MkdirTemp(root, ".loomloom-skill-backup-")
		if err != nil {
			return nil, fmt.Errorf("create skill backup: %w", err)
		}
		if err := os.Remove(backup); err != nil {
			return nil, fmt.Errorf("prepare skill backup: %w", err)
		}
		if err := os.Rename(target, backup); err != nil {
			return nil, fmt.Errorf("back up existing skill: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect existing skill directory: %w", err)
	}
	rollback := func() {
		_ = os.RemoveAll(target)
		if backup != "" {
			_ = os.Rename(backup, target)
		}
	}

	if err := os.Rename(stagedDir, target); err != nil {
		rollback()
		return nil, fmt.Errorf("install skill package: %w", err)
	}
	if backup != "" {
		_ = os.RemoveAll(backup)
	}
	bestEffortSyncDir(root)
	return &PackageInstallResult{Installed: true, Updated: hadPrevious, SkillName: skillName, Dir: target, ArchiveHash: actualHash}, nil
}

func ensureTargetOwnedBySource(target, sourceRef string) error {
	sourceRef = strings.TrimSpace(sourceRef)
	if sourceRef == "" {
		return nil
	}
	contents, err := os.ReadFile(filepath.Join(target, packageMarkerName))
	if os.IsNotExist(err) {
		return fmt.Errorf("skill name conflict: %s already exists and is not managed by LoomLoom", filepath.Base(target))
	}
	if err != nil {
		return fmt.Errorf("read existing package marker: %w", err)
	}
	marker, ok := parsePackageMarker(contents)
	if !ok {
		return fmt.Errorf("skill name conflict: %s has an invalid LoomLoom package marker", filepath.Base(target))
	}
	if marker.Source != sourceRef {
		return fmt.Errorf("skill name conflict: %s belongs to %s", filepath.Base(target), marker.Source)
	}
	return nil
}

const packageMarkerName = ".loomloom-package.json"

type packageMarker struct {
	SchemaVersion int    `json:"schemaVersion"`
	Source        string `json:"source"`
	ArchiveHash   string `json:"archiveHash"`
}

// FindInstalledPackage returns the installed package for one stable backend
// source when its recorded archive hash still matches the current server hash.
// The marker is deliberately stored inside the installed Skill directory so
// deleting that directory also deletes the local version state.
func FindInstalledPackage(skillRoot, sourceRef, archiveHash string) (*PackageInstallResult, bool, error) {
	root, err := absoluteSkillRoot(skillRoot)
	if err != nil {
		return nil, false, err
	}
	sourceRef = strings.TrimSpace(sourceRef)
	archiveHash = normalizeArchiveHash(archiveHash)
	if sourceRef == "" || archiveHash == "" {
		return nil, false, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, false, fmt.Errorf("read skill root: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		marker, err := os.ReadFile(filepath.Join(dir, packageMarkerName))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, false, fmt.Errorf("read installed package marker: %w", err)
		}
		if len(marker) > 1024 {
			return nil, false, errors.New("installed package marker is too large")
		}
		marked, ok := parsePackageMarker(marker)
		if !ok || marked.Source != sourceRef {
			continue
		}
		if normalizeArchiveHash(marked.ArchiveHash) != archiveHash {
			return nil, false, nil
		}
		return &PackageInstallResult{
			Installed: true, Unchanged: true, SkillName: entry.Name(), Dir: dir, ArchiveHash: archiveHash,
		}, true, nil
	}
	return nil, false, nil
}

func writePackageMarker(dir, sourceRef, archiveHash string) error {
	sourceRef = strings.TrimSpace(sourceRef)
	if sourceRef == "" {
		return nil
	}
	if strings.ContainsAny(sourceRef, "\r\n") {
		return errors.New("package source reference contains a newline")
	}
	contents, err := json.MarshalIndent(packageMarker{
		SchemaVersion: 1,
		Source:        sourceRef,
		ArchiveHash:   normalizeArchiveHash(archiveHash),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode installed package marker: %w", err)
	}
	contents = append(contents, '\n')
	if err := os.WriteFile(filepath.Join(dir, packageMarkerName), contents, 0o600); err != nil {
		return fmt.Errorf("write installed package marker: %w", err)
	}
	return nil
}

func parsePackageMarker(value []byte) (packageMarker, bool) {
	var marker packageMarker
	if err := json.Unmarshal(value, &marker); err != nil {
		return packageMarker{}, false
	}
	marker.Source = strings.TrimSpace(marker.Source)
	marker.ArchiveHash = normalizeArchiveHash(marker.ArchiveHash)
	if marker.SchemaVersion != 1 || marker.Source == "" || marker.ArchiveHash == "" {
		return packageMarker{}, false
	}
	return marker, true
}

func stagePackage(root string, archive []byte) (string, string, func(), error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return "", "", nil, fmt.Errorf("read package ZIP: %w", err)
	}
	if len(reader.File) == 0 || len(reader.File) > 100 {
		return "", "", nil, errors.New("package ZIP must contain between 1 and 100 entries")
	}
	stage, err := os.MkdirTemp(root, ".loomloom-skill-package-")
	if err != nil {
		return "", "", nil, fmt.Errorf("create package staging directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(stage) }
	var top string
	var total int64
	for _, file := range reader.File {
		name := file.Name
		if name == "" || strings.Contains(name, "\\") || strings.ContainsRune(name, 0) || strings.HasPrefix(name, "/") {
			cleanup()
			return "", "", nil, fmt.Errorf("unsafe package ZIP entry %q", name)
		}
		parts := strings.Split(strings.TrimSuffix(name, "/"), "/")
		if len(parts) == 0 || parts[0] == "" || parts[0] == "." || parts[0] == ".." {
			cleanup()
			return "", "", nil, fmt.Errorf("unsafe package ZIP entry %q", name)
		}
		for _, part := range parts {
			if part == "" || part == "." || part == ".." {
				cleanup()
				return "", "", nil, fmt.Errorf("unsafe package ZIP entry %q", name)
			}
		}
		if len(parts) == 2 && parts[1] == packageMarkerName {
			cleanup()
			return "", "", nil, fmt.Errorf("package ZIP must not contain reserved file %q", packageMarkerName)
		}
		if top == "" {
			top = parts[0]
		} else if top != parts[0] {
			cleanup()
			return "", "", nil, errors.New("package ZIP must have one top-level directory")
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			cleanup()
			return "", "", nil, fmt.Errorf("package ZIP entry %q is a symlink", name)
		}
		if file.UncompressedSize64 > 2<<20 {
			cleanup()
			return "", "", nil, fmt.Errorf("package ZIP entry %q exceeds 2 MiB", name)
		}
		total += int64(file.UncompressedSize64)
		if total > 15<<20 {
			cleanup()
			return "", "", nil, errors.New("package ZIP expands beyond 15 MiB")
		}
		destination := filepath.Join(stage, filepath.FromSlash(name))
		if !isWithin(stage, destination) {
			cleanup()
			return "", "", nil, fmt.Errorf("unsafe package ZIP entry %q", name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				cleanup()
				return "", "", nil, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			cleanup()
			return "", "", nil, err
		}
		input, err := file.Open()
		if err != nil {
			cleanup()
			return "", "", nil, err
		}
		contents, err := io.ReadAll(input)
		closeErr := input.Close()
		if err != nil || closeErr != nil {
			cleanup()
			return "", "", nil, fmt.Errorf("read package ZIP entry %q: %w", name, firstErr(err, closeErr))
		}
		if err := os.WriteFile(destination, contents, file.Mode()); err != nil {
			cleanup()
			return "", "", nil, err
		}
	}
	if top == "" {
		cleanup()
		return "", "", nil, errors.New("package ZIP has no skill directory")
	}
	skillDir := filepath.Join(stage, top)
	if err := validateSkillPackageDir(skillDir, top); err != nil {
		cleanup()
		return "", "", nil, err
	}
	return top, skillDir, cleanup, nil
}

func validateSkillPackageDir(dir, name string) error {
	contents, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("package must contain %s/SKILL.md: %w", name, err)
	}
	if len(contents) > 500<<10 {
		return errors.New("package SKILL.md exceeds 500 KiB")
	}
	lines := strings.Split(string(contents), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return errors.New("package SKILL.md must start with frontmatter")
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		if key, value, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(key) == "name" && strings.Trim(strings.TrimSpace(value), "\"'") == name {
			return nil
		}
	}
	return fmt.Errorf("package SKILL.md frontmatter name must equal %q", name)
}

func absoluteSkillRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", errors.New("--skill-root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if info, err := os.Lstat(abs); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("skill root must not be a symlink")
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", fmt.Errorf("create skill root: %w", err)
	}
	return abs, nil
}

func archiveHash(bytes []byte) string {
	sum := sha256.Sum256(bytes)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func normalizeArchiveHash(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "sha256:") {
		value = "sha256:" + value
	}
	return value
}
func isWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
func firstErr(a, b error) error {
	if a != nil {
		return a
	}
	return b
}
