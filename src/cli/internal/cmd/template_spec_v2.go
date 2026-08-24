package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// loadTemplateSpecTransportFile performs transport-safe JSON parsing only.
// Semantic and authority validation belongs to the selected LoomLoom server.
func loadTemplateSpecTransportFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("template spec file is empty")
	}
	var instance any
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&instance); err != nil {
		return nil, fmt.Errorf("parse TemplateSpec JSON: %w", err)
	}
	if err := requireTemplateSpecEOF(decoder); err != nil {
		return nil, err
	}
	if _, ok := instance.(map[string]any); !ok {
		return nil, errors.New("TemplateSpec JSON must be an object")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return nil, fmt.Errorf("compact TemplateSpec JSON: %w", err)
	}
	return compact.Bytes(), nil
}

func requireTemplateSpecEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("TemplateSpec file contains multiple JSON values")
		}
		return fmt.Errorf("parse trailing TemplateSpec JSON: %w", err)
	}
	return nil
}
