package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	templatespecdocs "github.com/cogfoundry-labs/loomloom/src/cli/internal/template_spec_docs"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

// These types and helpers validate bundled documentation fixtures in tests.
// Runtime commands deliberately use only loadTemplateSpecTransportFile and
// delegate all semantic and authority validation to the selected server.
type templateSpecEnvelope struct {
	Meta           templateSpecMeta           `json:"meta"`
	TemplateInputs map[string]json.RawMessage `json:"templateInputs,omitempty"`
	Steps          []templateSpecStep         `json:"steps"`
	Workbook       map[string]json.RawMessage `json:"workbook"`
}

type templateSpecMeta struct {
	Name string `json:"name"`
}

type templateSpecStep struct {
	StepID        string                              `json:"stepId"`
	DependsOn     []string                            `json:"dependsOn,omitempty"`
	InputBindings map[string]templateSpecInputBinding `json:"inputBindings,omitempty"`
}

type templateSpecInputBinding struct {
	Source  string `json:"source"`
	StepID  string `json:"stepId,omitempty"`
	Compose struct {
		Parts []json.RawMessage `json:"parts"`
	} `json:"compose,omitempty"`
}

var (
	templateSpecFixtureSchemaOnce sync.Once
	templateSpecFixtureSchema     *jsonschema.Schema
	templateSpecFixtureSchemaErr  error
)

func loadTemplateSpecFile(path string) (templateSpecEnvelope, []byte, error) {
	raw, err := loadTemplateSpecTransportFile(path)
	if err != nil {
		return templateSpecEnvelope{}, nil, err
	}
	var instance any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&instance); err != nil {
		return templateSpecEnvelope{}, nil, err
	}
	schema, err := compiledTemplateSpecFixtureSchema()
	if err != nil {
		return templateSpecEnvelope{}, nil, err
	}
	if err := schema.Validate(instance); err != nil {
		return templateSpecEnvelope{}, nil, fmt.Errorf("TemplateSpec v2 schema validation failed: %w", err)
	}
	var spec templateSpecEnvelope
	if err := json.Unmarshal(raw, &spec); err != nil {
		return templateSpecEnvelope{}, nil, err
	}
	if strings.TrimSpace(spec.Meta.Name) == "" {
		return templateSpecEnvelope{}, nil, errors.New("TemplateSpec meta.name is required")
	}
	for index, step := range spec.Steps {
		dependencies := make(map[string]bool, len(step.DependsOn))
		for _, dependency := range step.DependsOn {
			dependencies[dependency] = true
		}
		for portID, binding := range step.InputBindings {
			if binding.Source == "stepOutput" && !dependencies[binding.StepID] {
				return templateSpecEnvelope{}, nil, fmt.Errorf("steps[%d].inputBindings[%q] step %q must appear in dependsOn", index, portID, binding.StepID)
			}
		}
	}
	return spec, raw, nil
}

func compiledTemplateSpecFixtureSchema() (*jsonschema.Schema, error) {
	templateSpecFixtureSchemaOnce.Do(func() {
		data, err := templatespecdocs.FS.ReadFile("generated/machine/template-spec.schema.json")
		if err != nil {
			templateSpecFixtureSchemaErr = fmt.Errorf("read embedded TemplateSpec v2 schema: %w", err)
			return
		}
		compiler := jsonschema.NewCompiler()
		const schemaURL = "https://loomloom.dev/schemas/template-spec-v2.json"
		if err := compiler.AddResource(schemaURL, bytes.NewReader(data)); err != nil {
			templateSpecFixtureSchemaErr = err
			return
		}
		templateSpecFixtureSchema, templateSpecFixtureSchemaErr = compiler.Compile(schemaURL)
	})
	return templateSpecFixtureSchema, templateSpecFixtureSchemaErr
}
