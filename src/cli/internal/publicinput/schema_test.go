package publicinput

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseSnapshotSnakeCaseCompleteContract(t *testing.T) {
	raw := json.RawMessage(`{
		"schema_version":"loom_market_public_input_schema_v1",
		"fields":[{
			"key":"stage",
			"label":"Funding stage",
			"description":"Choose one",
			"required":true,
			"value_type":"enum",
			"enum_values":["Seed","Series A"],
			"accepted_mime_types":["text/plain"],
			"multi_value":false,
			"max_values":0,
			"order":2,
			"default_value":"Seed",
			"source_kind":"user_input",
			"presentation":{"widget":"select","placeholder":"Pick","hint":"One","examples":["Seed"]},
			"future_field":{"kept":"in raw"}
		}],
		"instructions":["One row per task"],
		"sample_rows":[{"stage":"Seed"}],
		"future_top_level":true
	}`)

	schema, err := ParseSnapshot(raw)
	if err != nil {
		t.Fatalf("ParseSnapshot() error = %v", err)
	}
	if schema.SchemaVersion != "loom_market_public_input_schema_v1" || len(schema.Fields) != 1 {
		t.Fatalf("schema = %#v", schema)
	}
	field := schema.Fields[0]
	if field.Key != "stage" || field.ValueType != "enum" || !reflect.DeepEqual(field.EnumValues, []string{"Seed", "Series A"}) {
		t.Fatalf("field = %#v", field)
	}
	if field.MaxValues != 0 || field.Order != 2 || field.DefaultValue != "Seed" || field.SourceKind != "user_input" {
		t.Fatalf("field constraints = %#v", field)
	}
	if field.Presentation == nil || field.Presentation.Widget != "select" || !reflect.DeepEqual(field.Presentation.Examples, []string{"Seed"}) {
		t.Fatalf("presentation = %#v", field.Presentation)
	}
	if !reflect.DeepEqual(schema.SampleRows, []map[string]string{{"stage": "Seed"}}) {
		t.Fatalf("sample rows = %#v", schema.SampleRows)
	}
}

func TestParseSnapshotLowerCamelCaseString(t *testing.T) {
	inner := `{
		"schemaVersion":"v1",
		"fields":[{
			"key":"stage",
			"valueType":"enum",
			"enumValues":["Seed","Series A"],
			"acceptedMimeTypes":["text/plain"],
			"multiValue":true,
			"maxValues":2,
			"defaultValue":"Seed",
			"sourceKind":"user_input"
		}],
		"sampleRows":[{"stage":"Seed"}]
	}`
	encoded, err := json.Marshal(inner)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := ParseSnapshot(encoded)
	if err != nil {
		t.Fatalf("ParseSnapshot() error = %v", err)
	}
	if schema.SchemaVersion != "v1" || len(schema.Fields) != 1 {
		t.Fatalf("schema = %#v", schema)
	}
	field := schema.Fields[0]
	if field.ValueType != "enum" ||
		!reflect.DeepEqual(field.EnumValues, []string{"Seed", "Series A"}) ||
		!reflect.DeepEqual(field.AcceptedMimeTypes, []string{"text/plain"}) ||
		!field.MultiValue || field.MaxValues != 2 ||
		field.DefaultValue != "Seed" || field.SourceKind != "user_input" {
		t.Fatalf("field = %#v", field)
	}
	if !reflect.DeepEqual(schema.SampleRows, []map[string]string{{"stage": "Seed"}}) {
		t.Fatalf("sample rows = %#v", schema.SampleRows)
	}
}

func TestParseSnapshotAcceptsEqualAliasesIncludingZeroValues(t *testing.T) {
	raw := json.RawMessage(`{"schema_version":"v1","schemaVersion":"v1","fields":[{"key":"stage","enum_values":[],"enumValues":[],"multi_value":false,"multiValue":false,"max_values":0,"maxValues":0}]}`)
	if _, err := ParseSnapshot(raw); err != nil {
		t.Fatalf("ParseSnapshot() error = %v", err)
	}
}

func TestParseSnapshotRejectsConflictingAliases(t *testing.T) {
	raw := json.RawMessage(`{"fields":[{"key":"stage","enum_values":[],"enumValues":["Seed"]}]}`)
	_, err := ParseSnapshot(raw)
	if err == nil || !strings.Contains(err.Error(), "conflicting values") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseSnapshotUsesNonNullAlias(t *testing.T) {
	raw := json.RawMessage(`{"fields":[{"key":"stage","enum_values":null,"enumValues":["Seed"]}]}`)
	schema, err := ParseSnapshot(raw)
	if err != nil {
		t.Fatalf("ParseSnapshot() error = %v", err)
	}
	if !reflect.DeepEqual(schema.Fields[0].EnumValues, []string{"Seed"}) {
		t.Fatalf("enum values = %#v", schema.Fields[0].EnumValues)
	}
}

func TestParseSnapshotRejectsInvalidRootAndNestedString(t *testing.T) {
	tests := []json.RawMessage{
		json.RawMessage(`[]`),
		json.RawMessage(`true`),
		json.RawMessage(`"\"{}\""`),
	}
	for _, raw := range tests {
		if _, err := ParseSnapshot(raw); err == nil {
			t.Fatalf("ParseSnapshot(%s) succeeded", raw)
		}
	}
}

func TestParseSnapshotPreservesFieldArrayOrder(t *testing.T) {
	raw := json.RawMessage(`{"fields":[{"key":"second","order":2},{"key":"first","order":1}]}`)
	schema, err := ParseSnapshot(raw)
	if err != nil {
		t.Fatalf("ParseSnapshot() error = %v", err)
	}
	if got := []string{schema.Fields[0].Key, schema.Fields[1].Key}; !reflect.DeepEqual(got, []string{"second", "first"}) {
		t.Fatalf("field order = %v", got)
	}
}

func TestParseSnapshotRejectsNonStringSampleRowValues(t *testing.T) {
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"fields":[{"key":"count"}],"sample_rows":[{"count":1}]}`),
		json.RawMessage(`{"fields":[{"key":"enabled"}],"sampleRows":[{"enabled":true}]}`),
	} {
		_, err := ParseSnapshot(raw)
		if err == nil || !strings.Contains(err.Error(), "sample_rows/sampleRows") {
			t.Fatalf("ParseSnapshot(%s) error = %v, want sample row type error", raw, err)
		}
	}
}
