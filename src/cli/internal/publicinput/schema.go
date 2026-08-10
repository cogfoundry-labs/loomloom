// Package publicinput parses the public input contract exposed by Market listings.
package publicinput

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Schema is the normalized public input schema.
type Schema struct {
	SchemaVersion string
	Fields        []Field
	Instructions  []string
	SampleRows    []map[string]string
}

// Field mirrors the backend's complete public field contract.
type Field struct {
	Key               string
	Label             string
	Description       string
	Required          bool
	ValueType         string
	EnumValues        []string
	AcceptedMimeTypes []string
	MultiValue        bool
	MaxValues         int32
	Order             int32
	DefaultValue      string
	SourceKind        string
	Presentation      *Presentation
}

// Presentation contains optional user-facing hints.
type Presentation struct {
	Widget      string   `json:"widget"`
	Placeholder string   `json:"placeholder"`
	Hint        string   `json:"hint"`
	Examples    []string `json:"examples"`
}

// ParseSnapshot accepts a schema object or one JSON string containing it.
// The backend uses snake_case; lowerCamelCase is defensive compatibility only.
func ParseSnapshot(raw json.RawMessage) (Schema, error) {
	data := bytes.TrimSpace(raw)
	if len(data) == 0 {
		return Schema{}, fmt.Errorf("input schema snapshot is empty")
	}
	if data[0] == '"' {
		var encoded string
		if err := json.Unmarshal(data, &encoded); err != nil {
			return Schema{}, fmt.Errorf("decode input schema snapshot string: %w", err)
		}
		data = bytes.TrimSpace([]byte(encoded))
		if len(data) == 0 {
			return Schema{}, fmt.Errorf("input schema snapshot is empty")
		}
	}
	if data[0] != '{' {
		return Schema{}, fmt.Errorf("input schema snapshot must decode to a JSON object")
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return Schema{}, fmt.Errorf("decode input schema snapshot object: %w", err)
	}
	if object == nil {
		return Schema{}, fmt.Errorf("input schema snapshot must decode to a JSON object")
	}

	var schema Schema
	if err := decodeAlias(object, "schema_version", "schemaVersion", &schema.SchemaVersion); err != nil {
		return Schema{}, err
	}
	if err := decodeOptional(object, "instructions", &schema.Instructions); err != nil {
		return Schema{}, err
	}
	if err := decodeAlias(object, "sample_rows", "sampleRows", &schema.SampleRows); err != nil {
		return Schema{}, err
	}

	var fields []json.RawMessage
	if err := decodeOptional(object, "fields", &fields); err != nil {
		return Schema{}, err
	}
	schema.Fields = make([]Field, 0, len(fields))
	for index, fieldRaw := range fields {
		field, err := parseField(fieldRaw)
		if err != nil {
			return Schema{}, fmt.Errorf("fields[%d]: %w", index, err)
		}
		schema.Fields = append(schema.Fields, field)
	}
	return schema, nil
}

func parseField(raw json.RawMessage) (Field, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return Field{}, fmt.Errorf("field must be a JSON object: %w", err)
	}
	if object == nil {
		return Field{}, fmt.Errorf("field must be a JSON object")
	}

	var field Field
	for _, item := range []struct {
		name string
		dest any
	}{
		{name: "key", dest: &field.Key},
		{name: "label", dest: &field.Label},
		{name: "description", dest: &field.Description},
		{name: "required", dest: &field.Required},
		{name: "order", dest: &field.Order},
		{name: "presentation", dest: &field.Presentation},
	} {
		if err := decodeOptional(object, item.name, item.dest); err != nil {
			return Field{}, err
		}
	}
	for _, item := range []struct {
		snake string
		camel string
		dest  any
	}{
		{snake: "value_type", camel: "valueType", dest: &field.ValueType},
		{snake: "enum_values", camel: "enumValues", dest: &field.EnumValues},
		{snake: "accepted_mime_types", camel: "acceptedMimeTypes", dest: &field.AcceptedMimeTypes},
		{snake: "multi_value", camel: "multiValue", dest: &field.MultiValue},
		{snake: "max_values", camel: "maxValues", dest: &field.MaxValues},
		{snake: "default_value", camel: "defaultValue", dest: &field.DefaultValue},
		{snake: "source_kind", camel: "sourceKind", dest: &field.SourceKind},
	} {
		if err := decodeAliasAny(object, item.snake, item.camel, item.dest); err != nil {
			return Field{}, err
		}
	}
	return field, nil
}

func decodeOptional(object map[string]json.RawMessage, name string, dest any) error {
	raw, ok := object[name]
	if !ok || isNull(raw) {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("decode %q: %w", name, err)
	}
	return nil
}

func decodeAlias[T any](object map[string]json.RawMessage, snake string, camel string, dest *T) error {
	return decodeAliasAny(object, snake, camel, dest)
}

func decodeAliasAny(object map[string]json.RawMessage, snake string, camel string, dest any) error {
	raw, ok, err := resolveAlias(object, snake, camel)
	if err != nil || !ok {
		return err
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("decode %q: %w", aliasLabel(snake, camel), err)
	}
	return nil
}

// Zero values count as present; only missing keys and null are unavailable.
func resolveAlias(object map[string]json.RawMessage, snake string, camel string) (json.RawMessage, bool, error) {
	snakeRaw, snakeOK := object[snake]
	camelRaw, camelOK := object[camel]
	snakeOK = snakeOK && !isNull(snakeRaw)
	camelOK = camelOK && !isNull(camelRaw)

	switch {
	case snakeOK && camelOK:
		equal, err := equalJSON(snakeRaw, camelRaw)
		if err != nil {
			return nil, false, fmt.Errorf("compare aliases %q and %q: %w", snake, camel, err)
		}
		if !equal {
			return nil, false, fmt.Errorf("input schema aliases %q and %q have conflicting values", snake, camel)
		}
		return snakeRaw, true, nil
	case snakeOK:
		return snakeRaw, true, nil
	case camelOK:
		return camelRaw, true, nil
	default:
		return nil, false, nil
	}
}

func equalJSON(left json.RawMessage, right json.RawMessage) (bool, error) {
	var leftValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		return false, err
	}
	var rightValue any
	if err := json.Unmarshal(right, &rightValue); err != nil {
		return false, err
	}
	return reflect.DeepEqual(leftValue, rightValue), nil
}

func isNull(raw json.RawMessage) bool {
	return strings.EqualFold(string(bytes.TrimSpace(raw)), "null")
}

func aliasLabel(snake string, camel string) string {
	if camel == "" {
		return snake
	}
	return snake + "/" + camel
}
