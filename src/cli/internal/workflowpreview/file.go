package workflowpreview

import (
	"errors"
	"io"
	"os"
	"unicode/utf8"
)

const MaxContentBytes = 1024 * 1024

func readFile(path string) (string, error) {
	// Stat first to avoid blocking on FIFOs/devices. Recheck the opened file to
	// detect replacements; a local file is not an untrusted remote upload.
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("file_missing")
		}
		return "", errors.New("file_unreadable")
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("file_not_regular")
	}
	if info.Size() > MaxContentBytes {
		return "", errors.New("file_too_large")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", errors.New("file_unreadable")
	}
	defer func() { _ = file.Close() }()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return "", errors.New("file_unreadable")
	}
	body, err := io.ReadAll(io.LimitReader(file, MaxContentBytes+1))
	if err != nil {
		return "", errors.New("file_unreadable")
	}
	if len(body) > MaxContentBytes {
		return "", errors.New("file_too_large")
	}
	after, err := file.Stat()
	current, pathErr := os.Stat(path)
	if err != nil || pathErr != nil || !os.SameFile(before, current) ||
		before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return "", errors.New("file_changing")
	}
	if !utf8.Valid(body) {
		return "", errors.New("file_invalid_utf8")
	}
	return string(body), nil
}
