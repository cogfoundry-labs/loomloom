package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cogfoundry-labs/loomloom/src/cli/internal/client"
	"github.com/spf13/cobra"
)

// Opt in with LOOMLOOM_DELIVERY_TEST_SERVER and LOOMLOOM_DELIVERY_TEST_TOKEN.
// Creates only dedicated test templates/versions, without running providers,
// changing existing templates, or publishing to Market. IDs are logged so the
// retained fixtures can be inspected; Public API has no template delete route.
func TestTemplateSpecDeliveryIntegration(t *testing.T) {
	server, token := os.Getenv("LOOMLOOM_DELIVERY_TEST_SERVER"), os.Getenv("LOOMLOOM_DELIVERY_TEST_TOKEN")
	if server == "" && token == "" {
		t.Skip("live delivery authoring test is opt-in")
	}
	if server == "" || token == "" {
		t.Fatal("both delivery test server and token are required")
	}
	opts := &rootOptions{server: server, token: token, output: "json", timeout: 60 * time.Second}
	run := func(command *cobra.Command, args ...string) (map[string]any, error) {
		var out, warnings bytes.Buffer
		command.SetOut(&out)
		command.SetErr(&warnings)
		command.SetArgs(args)
		if err := command.Execute(); err != nil {
			return nil, err
		}
		var result map[string]any
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			return nil, fmt.Errorf("decode command output: %w", err)
		}
		return result, nil
	}
	must := func(command *cobra.Command, args ...string) map[string]any {
		t.Helper()
		result, err := run(command, args...)
		if err != nil {
			t.Fatal(strings.ReplaceAll(err.Error(), token, "[REDACTED]"))
		}
		return result
	}
	expectRejected := func(command *cobra.Command, args ...string) {
		t.Helper()
		_, err := run(command, args...)
		var requestError client.RequestError
		if !errors.As(err, &requestError) || requestError.StatusCode != http.StatusBadRequest || !strings.Contains(requestError.Body, "deliveryOutputs") {
			t.Fatalf("expected deliveryOutputs HTTP 400, got %s", strings.ReplaceAll(fmt.Sprint(err), token, "[REDACTED]"))
		}
	}
	fixture := filepath.Join(findRepoRoot(t), "docs/ir-spec/en/examples/valid/delivery-code-outputs.json")
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	declaration := spec["deliveryOutputs"]
	path := filepath.Join(t.TempDir(), "spec.json")
	write := func() {
		t.Helper()
		data, err := json.Marshal(spec)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Fail on authentication/capability errors before creating remote fixtures.
	delete(spec, "deliveryOutputs")
	write()
	oldCheck := must(newTemplateSpecCheckCmd(opts), path)
	spec["deliveryOutputs"] = declaration
	write()
	newCheck := must(newTemplateSpecCheckCmd(opts), path)
	if oldCheck["definitionHash"] == nil || newCheck["definitionHash"] == nil || oldCheck["definitionHash"] == newCheck["definitionHash"] {
		t.Fatal("delivery declaration did not change definition hash")
	}
	items := declaration.(map[string]any)["items"].([]any)
	invalid := map[string]any{
		"empty-object":     map[string]any{},
		"empty-items":      map[string]any{"schemaVersion": 1, "items": []any{}},
		"wrong-version":    map[string]any{"schemaVersion": 2, "items": items},
		"missing-port":     map[string]any{"schemaVersion": 1, "items": []any{map[string]any{"key": "bad", "label": "Bad", "stepId": "stp_code001", "portId": "missing"}}},
		"missing-step":     map[string]any{"schemaVersion": 1, "items": []any{map[string]any{"key": "bad", "label": "Bad", "stepId": "stp_missing1", "portId": "file"}}},
		"duplicate-key":    map[string]any{"schemaVersion": 1, "items": []any{items[0], map[string]any{"key": "resultFile", "label": "Text", "stepId": "stp_code001", "portId": "text"}}},
		"duplicate-source": map[string]any{"schemaVersion": 1, "items": []any{items[0], map[string]any{"key": "anotherFile", "label": "File", "stepId": "stp_code001", "portId": "file"}}},
		"blank-label":      map[string]any{"schemaVersion": 1, "items": []any{map[string]any{"key": "bad", "label": " ", "stepId": "stp_code001", "portId": "file"}}},
	}
	for name, value := range invalid {
		spec["deliveryOutputs"] = value
		write()
		expectRejected(newTemplateSpecCheckCmd(opts), path)
		t.Logf("rejected %s", name)
	}
	read := func(templateID, versionID string, want any) map[string]any {
		t.Helper()
		target := filepath.Join(t.TempDir(), "export.json")
		must(newTemplateSpecGetVersionCmd(opts), templateID, versionID, "--output-file", target)
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result["deliveryOutputs"], want) {
			t.Fatalf("read-back delivery differs for version %s", versionID)
		}
		return result
	}
	for _, declared := range []bool{false, true} {
		spec["meta"].(map[string]any)["name"] = fmt.Sprintf("CLI delivery regression %s declared=%t", time.Now().UTC().Format("20060102T150405.000000000"), declared)
		var want any
		if declared {
			want = declaration
			spec["deliveryOutputs"] = want
		} else {
			delete(spec, "deliveryOutputs")
		}
		write()
		created := must(newTemplateSpecCreateCmd(opts), path)
		templateID, ok := created["templateId"].(string)
		if !ok || templateID == "" {
			t.Fatal("create omitted templateId")
		}
		versionID, ok := created["versionId"].(string)
		if !ok || versionID == "" {
			t.Fatal("create omitted versionId")
		}
		t.Logf("created dedicated fixture template=%s version=%s declared=%t", templateID, versionID, declared)
		exported := read(templateID, versionID, want)
		// Edit an exported spec, then save a new version with a different declaration.
		var edited map[string]any
		data, err := json.Marshal(exported)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &edited); err != nil {
			t.Fatal(err)
		}
		if declared {
			edited["deliveryOutputs"].(map[string]any)["items"].([]any)[0].(map[string]any)["label"] = "最终结果文件"
		} else {
			edited["deliveryOutputs"] = declaration
		}
		spec = edited
		write()
		updated := must(newTemplateSpecCreateVersionCmd(opts), templateID, path)
		nextID, ok := updated["versionId"].(string)
		if !ok || nextID == "" || nextID == versionID {
			t.Fatal("new immutable version ID missing")
		}
		wantUpdated := spec["deliveryOutputs"]
		read(templateID, nextID, wantUpdated)
		read(templateID, versionID, want)
		details := must(newTemplateSpecGetCmd(opts), templateID)
		if details["latestVersionId"] != nextID || details["publishedVersionId"] != nextID {
			t.Fatal("latest/published version pointers differ from newly saved version")
		}
		// Server validation must reject bad saves without appending a version.
		before := must(newTemplateSpecVersionsCmd(opts), templateID)
		spec["deliveryOutputs"] = invalid["missing-port"]
		write()
		expectRejected(newTemplateSpecCreateVersionCmd(opts), templateID, path)
		after := must(newTemplateSpecVersionsCmd(opts), templateID)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("failed validation changed version list")
		}
		// Removing an optional declaration affects only another new version.
		spec = read(templateID, nextID, wantUpdated)
		delete(spec, "deliveryOutputs")
		write()
		removed := must(newTemplateSpecCreateVersionCmd(opts), templateID, path)
		removedID, ok := removed["versionId"].(string)
		if !ok || removedID == "" || removedID == nextID {
			t.Fatal("removal did not create an immutable version")
		}
		read(templateID, removedID, nil)
		read(templateID, nextID, wantUpdated)
		t.Logf("verified export/edit/new-version/history and rejection without mutation: template=%s newVersion=%s", templateID, nextID)
	}
}

// Run an explicitly supplied, dedicated delivery fixture. These extra variables
// opt into hosted execution separately from the authoring-only test above.
func TestTemplateSpecDeliveryExecutionIntegration(t *testing.T) {
	templateID := os.Getenv("LOOMLOOM_DELIVERY_RUN_TEMPLATE")
	versionID := os.Getenv("LOOMLOOM_DELIVERY_RUN_VERSION")
	inputID := os.Getenv("LOOMLOOM_DELIVERY_RUN_INPUT")
	if templateID == "" && versionID == "" && inputID == "" {
		t.Skip("hosted delivery execution test is opt-in")
	}
	server, token := os.Getenv("LOOMLOOM_DELIVERY_TEST_SERVER"), os.Getenv("LOOMLOOM_DELIVERY_TEST_TOKEN")
	if templateID == "" || versionID == "" || inputID == "" || server == "" || token == "" {
		t.Fatal("delivery execution requires server, token, template, version and input")
	}
	opts := &rootOptions{server: server, token: token, output: "json", timeout: 60 * time.Second}
	run := func(command *cobra.Command, args ...string) map[string]any {
		t.Helper()
		var out bytes.Buffer
		command.SetOut(&out)
		command.SetErr(&out)
		command.SetArgs(args)
		if err := command.Execute(); err != nil {
			t.Fatal(strings.ReplaceAll(err.Error(), token, "[REDACTED]"))
		}
		var result map[string]any
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	version := run(newTemplateSpecGetVersionCmd(opts), templateID, versionID)
	spec, ok := version["canonicalSpec"].(map[string]any)
	if !ok || spec["deliveryOutputs"] == nil {
		t.Fatal("execution fixture must explicitly declare deliveryOutputs")
	}
	run(newTemplateSpecPrecheckCmd(opts), templateID, "--version-id", versionID, "--input-file-id", inputID)
	submitted := run(newTemplateSpecRunCmd(opts), templateID, "--version-id", versionID, "--input-file-id", inputID, "--client-request-id", fmt.Sprintf("cli-delivery-execution-%d", time.Now().UnixNano()))
	runID, ok := submitted["runId"].(string)
	if !ok || runID == "" {
		t.Fatal("submission omitted runId")
	}
	t.Logf("submitted delivery fixture run=%s", runID)
	completed := run(newRunWatchCmd(opts), runID, "--interval", "2s", "--max-wait", "2m")
	if completed["status"] != "completed" {
		t.Fatalf("run did not complete: %v", completed["status"])
	}
	run(newRunResultRowsCmd(opts), runID)
	target := filepath.Join(t.TempDir(), "result.xlsx")
	run(newRunResultWorkbookCmd(opts), runID, "--output-file", target)
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("PK")) {
		t.Fatal("result workbook is not an XLSX archive")
	}
	run(newArtifactListCmd(opts), runID)
	run(newArtifactDownloadCmd(opts), runID, "--output-dir", filepath.Join(t.TempDir(), "artifacts"))
}
