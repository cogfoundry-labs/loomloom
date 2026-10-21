// Package viewerassets embeds a build of the workbench viewer, not a second UI implementation.
package viewerassets

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
)

//go:embed viewer.zip
var archive []byte

func Open() (fs.FS, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	raw, err := fs.ReadFile(reader, "manifest.json")
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Protocol string            `json:"protocol"`
		Files    map[string]string `json:"files"`
	}
	if json.Unmarshal(raw, &manifest) != nil || manifest.Protocol != "local-workflow-preview/v1" || manifest.Files["viewer.html"] == "" {
		return nil, fmt.Errorf("invalid embedded viewer manifest")
	}
	for _, file := range reader.File {
		if file.Name == "manifest.json" {
			continue
		}
		expected, ok := manifest.Files[file.Name]
		if !ok || !fs.ValidPath(file.Name) || file.UncompressedSize64 > 16*1024*1024 {
			return nil, fmt.Errorf("invalid embedded viewer asset")
		}
		stream, err := file.Open()
		if err != nil {
			return nil, err
		}
		hash := sha256.New()
		_, err = io.Copy(hash, io.LimitReader(stream, 16*1024*1024+1))
		_ = stream.Close()
		if err != nil || hex.EncodeToString(hash.Sum(nil)) != expected {
			return nil, fmt.Errorf("embedded viewer checksum mismatch")
		}
	}
	for name := range manifest.Files {
		if _, err := fs.Stat(reader, name); err != nil {
			return nil, fmt.Errorf("missing embedded viewer asset")
		}
	}
	return reader, nil
}
