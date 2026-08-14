package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	templatespecdocs "github.com/cogfoundry-labs/loomloom/src/cli/internal/template_spec_docs"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

type templateSpecMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type templateSpecEnvelope struct {
	Meta           templateSpecMeta             `json:"meta"`
	TemplateInputs map[string]templateSpecInput `json:"templateInputs,omitempty"`
	Steps          []templateSpecStep           `json:"steps"`
	Workbook       *templateSpecWorkbook        `json:"workbook"`
}

type templateSpecInput struct {
	Kind              string          `json:"kind"`
	ValueType         string          `json:"valueType,omitempty"`
	Required          bool            `json:"required,omitempty"`
	BlankPolicy       string          `json:"blankPolicy"`
	Constraints       json.RawMessage `json:"constraints,omitempty"`
	AcceptedMIMETypes []string        `json:"acceptedMimeTypes,omitempty"`
	MinItems          int             `json:"minItems,omitempty"`
	MaxItems          int             `json:"maxItems,omitempty"`
}

type templateSpecStep struct {
	StepID           string                              `json:"stepId"`
	DependsOn        []string                            `json:"dependsOn,omitempty"`
	TriggerPolicy    string                              `json:"triggerPolicy,omitempty"`
	ExecutionBinding templateSpecExecutionBinding        `json:"executionBinding"`
	ModelSelection   *templateSpecModelSelection         `json:"modelSelection,omitempty"`
	InputBindings    map[string]templateSpecInputBinding `json:"inputBindings,omitempty"`
}

type templateSpecExecutionBinding struct {
	Kind              string `json:"kind"`
	SubjectRevisionID string `json:"subjectRevisionId,omitempty"`
	ProfileID         string `json:"profileId,omitempty"`
	ProfileRevision   string `json:"profileRevision,omitempty"`
}

type templateSpecModelSelection struct {
	Source         string `json:"source"`
	InputKey       string `json:"inputKey"`
	DefaultModelID string `json:"defaultModelId"`
}

type templateSpecInputBinding struct {
	Source     string                   `json:"source"`
	InputKey   string                   `json:"inputKey,omitempty"`
	StepID     string                   `json:"stepId,omitempty"`
	PortID     string                   `json:"portId,omitempty"`
	Compose    *templateSpecComposition `json:"compose,omitempty"`
	Sequence   *templateSpecSequence    `json:"sequence,omitempty"`
	Merge      *templateSpecMerge       `json:"merge,omitempty"`
	ContextKey string                   `json:"contextKey,omitempty"`
}

type templateSpecComposition struct {
	Parts []templateSpecCompositionPart `json:"parts"`
}

type templateSpecCompositionPart struct {
	Source   string `json:"source"`
	InputKey string `json:"inputKey,omitempty"`
}

type templateSpecSequence struct {
	Items []templateSpecSequenceItem `json:"items"`
}

type templateSpecSequenceItem struct {
	Position int                            `json:"position"`
	Source   templateSpecSequenceItemSource `json:"source"`
}

type templateSpecSequenceItemSource struct {
	Source   string `json:"source"`
	InputKey string `json:"inputKey,omitempty"`
	StepID   string `json:"stepId,omitempty"`
	PortID   string `json:"portId,omitempty"`
}

type templateSpecMerge struct {
	Sources []templateSpecMergeSource `json:"sources"`
}

type templateSpecMergeSource struct {
	Source   string `json:"source"`
	InputKey string `json:"inputKey,omitempty"`
	StepID   string `json:"stepId,omitempty"`
	PortID   string `json:"portId,omitempty"`
}

type templateSpecWorkbook struct {
	SampleRows []templateSpecSampleRow `json:"sampleRows,omitempty"`
}

type templateSpecSampleRow struct {
	Values map[string]json.RawMessage `json:"values"`
}

var (
	templateSpecSchemaOnce sync.Once
	templateSpecSchema     *jsonschema.Schema
	templateSpecSchemaErr  error
)

func loadTemplateSpecFile(path string) (templateSpecEnvelope, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return templateSpecEnvelope{}, nil, fmt.Errorf("read %s: %w", path, err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return templateSpecEnvelope{}, nil, errors.New("template spec file is empty")
	}

	var instance any
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&instance); err != nil {
		return templateSpecEnvelope{}, nil, fmt.Errorf("parse TemplateSpec JSON: %w", err)
	}
	if err := requireTemplateSpecEOF(decoder); err != nil {
		return templateSpecEnvelope{}, nil, err
	}
	schema, err := compiledTemplateSpecV2Schema()
	if err != nil {
		return templateSpecEnvelope{}, nil, err
	}
	if err := schema.Validate(instance); err != nil {
		return templateSpecEnvelope{}, nil, fmt.Errorf("TemplateSpec v2 schema validation failed: %w", err)
	}

	var spec templateSpecEnvelope
	if err := json.Unmarshal(trimmed, &spec); err != nil {
		return templateSpecEnvelope{}, nil, fmt.Errorf("decode TemplateSpec v2: %w", err)
	}
	if err := validateTemplateSpecV2Local(spec); err != nil {
		return templateSpecEnvelope{}, nil, err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return templateSpecEnvelope{}, nil, fmt.Errorf("compact TemplateSpec v2 JSON: %w", err)
	}
	return spec, compact.Bytes(), nil
}

func compiledTemplateSpecV2Schema() (*jsonschema.Schema, error) {
	templateSpecSchemaOnce.Do(func() {
		data, err := templatespecdocs.FS.ReadFile("generated/machine/template-spec.schema.json")
		if err != nil {
			templateSpecSchemaErr = fmt.Errorf("read embedded TemplateSpec v2 schema: %w", err)
			return
		}
		compiler := jsonschema.NewCompiler()
		const schemaURL = "https://loomloom.dev/schemas/template-spec-v2.json"
		if err := compiler.AddResource(schemaURL, bytes.NewReader(data)); err != nil {
			templateSpecSchemaErr = fmt.Errorf("load embedded TemplateSpec v2 schema: %w", err)
			return
		}
		templateSpecSchema, templateSpecSchemaErr = compiler.Compile(schemaURL)
	})
	if templateSpecSchemaErr != nil {
		return nil, templateSpecSchemaErr
	}
	return templateSpecSchema, nil
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

var templateSpecStepIDPattern = regexp.MustCompile(`^stp_[0-9a-z]{6,10}$`)

func validateTemplateSpecV2Local(spec templateSpecEnvelope) error {
	if strings.TrimSpace(spec.Meta.Name) == "" {
		return errors.New("TemplateSpec meta.name is required")
	}
	if len(spec.Steps) == 0 {
		return errors.New("TemplateSpec steps must not be empty")
	}
	if spec.Workbook == nil {
		return errors.New("TemplateSpec workbook is required")
	}
	for rowIndex, row := range spec.Workbook.SampleRows {
		for inputKey := range row.Values {
			if _, ok := spec.TemplateInputs[inputKey]; !ok {
				return fmt.Errorf("workbook.sampleRows[%d] references unknown template input %q", rowIndex, inputKey)
			}
		}
	}

	steps := make(map[string]templateSpecStep, len(spec.Steps))
	for index, step := range spec.Steps {
		if !templateSpecStepIDPattern.MatchString(step.StepID) {
			return fmt.Errorf("steps[%d].stepId %q must match stp_<6-10 base36 chars>", index, step.StepID)
		}
		if _, duplicate := steps[step.StepID]; duplicate {
			return fmt.Errorf("steps[%d].stepId %q is duplicated", index, step.StepID)
		}
		steps[step.StepID] = step
	}
	for index, step := range spec.Steps {
		if err := validateTemplateSpecStepV2Local(index, step, spec.TemplateInputs, steps); err != nil {
			return err
		}
	}
	return validateTemplateSpecAcyclic(spec.Steps)
}

func validateTemplateSpecStepV2Local(index int, step templateSpecStep, inputs map[string]templateSpecInput, steps map[string]templateSpecStep) error {
	path := fmt.Sprintf("steps[%d]", index)
	dependencies := make(map[string]bool, len(step.DependsOn))
	for _, dependency := range step.DependsOn {
		if _, ok := steps[dependency]; !ok {
			return fmt.Errorf("%s.dependsOn references unknown step %q", path, dependency)
		}
		dependencies[dependency] = true
	}
	if step.ExecutionBinding.Kind == "fixedModelContract" && step.ModelSelection != nil {
		return fmt.Errorf("%s.modelSelection is not allowed for fixedModelContract", path)
	}
	if step.ExecutionBinding.Kind == "capabilityProfile" {
		if step.ModelSelection == nil {
			return fmt.Errorf("%s.modelSelection is required for capabilityProfile", path)
		}
		input, ok := inputs[step.ModelSelection.InputKey]
		if !ok || input.Kind != "value" || input.ValueType != "string" || input.Required || input.BlankPolicy != "omit" {
			return fmt.Errorf("%s.modelSelection.inputKey must reference an optional string Template Input", path)
		}
	}
	for portID, binding := range step.InputBindings {
		if err := validateTemplateSpecBindingV2Local(path, portID, binding, inputs, steps, dependencies); err != nil {
			return err
		}
	}
	return nil
}

func validateTemplateSpecBindingV2Local(path, portID string, binding templateSpecInputBinding, inputs map[string]templateSpecInput, steps map[string]templateSpecStep, dependencies map[string]bool) error {
	bindingPath := fmt.Sprintf("%s.inputBindings[%q]", path, portID)
	switch binding.Source {
	case "templateInput":
		if _, ok := inputs[binding.InputKey]; !ok {
			return fmt.Errorf("%s references unknown template input %q", bindingPath, binding.InputKey)
		}
	case "stepOutput":
		if _, ok := steps[binding.StepID]; !ok {
			return fmt.Errorf("%s references unknown step %q", bindingPath, binding.StepID)
		}
		if !dependencies[binding.StepID] {
			return fmt.Errorf("%s source step %q must appear in dependsOn", bindingPath, binding.StepID)
		}
	case "composeValue":
		for partIndex, part := range binding.Compose.Parts {
			if part.Source == "templateInput" {
				input, ok := inputs[part.InputKey]
				if !ok || input.Kind != "value" || input.ValueType != "string" {
					return fmt.Errorf("%s.compose.parts[%d] must reference a string Template Input", bindingPath, partIndex)
				}
			}
		}
	case "sequence":
		for itemIndex, item := range binding.Sequence.Items {
			if err := validateTemplateSpecNestedSource(bindingPath+fmt.Sprintf(".sequence.items[%d]", itemIndex), item.Source.Source, item.Source.InputKey, item.Source.StepID, inputs, steps, dependencies); err != nil {
				return err
			}
		}
	case "merge":
		for sourceIndex, source := range binding.Merge.Sources {
			if err := validateTemplateSpecNestedSource(bindingPath+fmt.Sprintf(".merge.sources[%d]", sourceIndex), source.Source, source.InputKey, source.StepID, inputs, steps, dependencies); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateTemplateSpecNestedSource(path, source, inputKey, stepID string, inputs map[string]templateSpecInput, steps map[string]templateSpecStep, dependencies map[string]bool) error {
	if source == "templateInput" {
		if _, ok := inputs[inputKey]; !ok {
			return fmt.Errorf("%s references unknown template input %q", path, inputKey)
		}
		return nil
	}
	if source == "stepOutput" {
		if _, ok := steps[stepID]; !ok {
			return fmt.Errorf("%s references unknown step %q", path, stepID)
		}
		if !dependencies[stepID] {
			return fmt.Errorf("%s source step %q must appear in dependsOn", path, stepID)
		}
	}
	return nil
}

func validateTemplateSpecAcyclic(steps []templateSpecStep) error {
	graph := make(map[string][]string, len(steps))
	for _, step := range steps {
		graph[step.StepID] = step.DependsOn
	}
	state := make(map[string]uint8, len(graph))
	var visit func(string) error
	visit = func(stepID string) error {
		if state[stepID] == 1 {
			return fmt.Errorf("TemplateSpec steps contain a dependency cycle at %q", stepID)
		}
		if state[stepID] == 2 {
			return nil
		}
		state[stepID] = 1
		for _, dependency := range graph[stepID] {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[stepID] = 2
		return nil
	}
	for stepID := range graph {
		if err := visit(stepID); err != nil {
			return err
		}
	}
	return nil
}

func countTemplateSpecBindings(spec templateSpecEnvelope) int {
	count := 0
	for _, step := range spec.Steps {
		count += len(step.InputBindings)
	}
	return count
}
