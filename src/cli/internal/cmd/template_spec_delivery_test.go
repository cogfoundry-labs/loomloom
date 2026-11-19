package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// Exercise the actual author commands together: validation, save, export, edit,
// and save again must preserve delivery selections without synthesizing them
// for historical templates. Semantic validation remains owned by the server.
func TestTemplateSpecDeliveryRoundTrip(t *testing.T) {
	for _, declared := range []bool{false, true} {
		t.Run(fmt.Sprintf("declared=%t", declared), func(t *testing.T) {
			isolateCmdConfigHome(t)
			raw, err := os.ReadFile(filepath.Join(findRepoRoot(t), "docs/ir-spec/en/examples/valid/delivery-code-outputs.json"))
			if err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatal(err)
			}
			if !declared {
				delete(want, "deliveryOutputs")
			}
			write := func(path string) {
				t.Helper()
				data, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(t.TempDir(), "draft.json")
			write(path)
			validations, saves := 0, 0
			var saved map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "POST /loom/v1/templateSpecs:validate", "POST /loom/v1/users/me/templates/tmpl_delivery/versions":
					var payload struct {
						CanonicalSpecV2 map[string]any `json:"canonicalSpecV2"`
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					if !reflect.DeepEqual(payload.CanonicalSpecV2, want) {
						t.Errorf("spec changed in transport: got %v want %v", payload.CanonicalSpecV2, want)
					}
					if r.URL.Path == "/loom/v1/templateSpecs:validate" {
						validations++
						fmt.Fprint(w, `{"valid":true,"definitionHash":"hash"}`)
					} else {
						saves++
						saved = payload.CanonicalSpecV2
						fmt.Fprint(w, `{"versionId":"ver_delivery","versionNumber":1,"definitionHash":"hash"}`)
					}
				case "POST /loom/v1/users/me/templates":
					fmt.Fprint(w, `{"templateId":"tmpl_delivery"}`)
				case "GET /loom/v1/users/me/templates/tmpl_delivery/versions/ver_delivery":
					json.NewEncoder(w).Encode(map[string]any{"templateId": "tmpl_delivery", "versionId": "ver_delivery", "specVersion": "template-spec/v2", "canonicalSpec": saved})
				case "GET /loom/v1/users/me/templates/tmpl_delivery/versions/ver_delivery/viewLink":
					fmt.Fprint(w, `{"templateId":"tmpl_delivery","versionId":"ver_delivery","viewUrl":"https://work.example/templates/tmpl_delivery/versions/ver_delivery"}`)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			opts := &rootOptions{server: server.URL + "/loom/v1", output: "json", timeout: time.Second}
			run := func(command *cobra.Command, args ...string) []byte {
				t.Helper()
				var out bytes.Buffer
				command.SetOut(&out)
				command.SetErr(&out)
				command.SetArgs(args)
				if err := command.Execute(); err != nil {
					t.Fatal(err)
				}
				return out.Bytes()
			}
			run(newTemplateSpecCheckCmd(opts), path)
			run(newTemplateSpecCreateCmd(opts), path)
			var got templateVersionSpecResponse
			if err := json.Unmarshal(run(newTemplateSpecGetVersionCmd(opts), "tmpl_delivery", "ver_delivery"), &got); err != nil {
				t.Fatal(err)
			}
			var spec map[string]any
			if err := json.Unmarshal(got.CanonicalSpec, &spec); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(spec, want) {
				t.Fatal("get-version output lost spec fields")
			}
			exported := filepath.Join(t.TempDir(), "exported.json")
			run(newTemplateSpecGetVersionCmd(opts), "tmpl_delivery", "ver_delivery", "--output-file", exported)
			data, err := os.ReadFile(exported)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &spec); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(spec, want) {
				t.Fatal("export lost spec fields")
			}
			if declared {
				items := want["deliveryOutputs"].(map[string]any)["items"].([]any)
				items[0].(map[string]any)["label"] = "最终结果文件"
			} else {
				want["meta"].(map[string]any)["name"] = "Edited legacy template"
			}
			write(exported)
			run(newTemplateSpecCreateVersionCmd(opts), "tmpl_delivery", exported)
			if validations != 3 || saves != 2 {
				t.Fatalf("validations=%d saves=%d", validations, saves)
			}
		})
	}
}

func TestTemplateSpecInvalidDeliveryStopsBeforeMutation(t *testing.T) {
	for _, delivery := range []string{`{}`, `{"schemaVersion":1,"items":[]}`, `{"schemaVersion":1,"items":[{"key":"result","label":"Result","stepId":"stp_code001","portId":"missing"}]}`} {
		for _, action := range []string{"check", "create", "create-version"} {
			t.Run(action+"/"+delivery, func(t *testing.T) {
				isolateCmdConfigHome(t)
				path := filepath.Join(t.TempDir(), "invalid.json")
				raw, err := os.ReadFile(filepath.Join(findRepoRoot(t), "docs/ir-spec/en/examples/valid/delivery-code-outputs.json"))
				if err != nil {
					t.Fatal(err)
				}
				var spec map[string]json.RawMessage
				if err := json.Unmarshal(raw, &spec); err != nil {
					t.Fatal(err)
				}
				spec["deliveryOutputs"] = json.RawMessage(delivery)
				raw, err = json.Marshal(spec)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				requests := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					if r.Method != "POST" || r.URL.Path != "/loom/v1/templateSpecs:validate" {
						t.Errorf("mutation after failed check: %s %s", r.Method, r.URL.Path)
					}
					var payload struct {
						CanonicalSpecV2 map[string]json.RawMessage `json:"canonicalSpecV2"`
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					if string(payload.CanonicalSpecV2["deliveryOutputs"]) != delivery {
						t.Error("invalid declaration stripped before server validation")
					}
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, `{"error":"deliveryOutputs: invalid declaration"}`)
				}))
				defer server.Close()
				opts := &rootOptions{server: server.URL + "/loom/v1", output: "json", timeout: time.Second}
				command, args := newTemplateSpecCheckCmd(opts), []string{path}
				if action == "create" {
					command = newTemplateSpecCreateCmd(opts)
				}
				if action == "create-version" {
					command, args = newTemplateSpecCreateVersionCmd(opts), []string{"tmpl_delivery", path}
				}
				command.SetOut(&bytes.Buffer{})
				command.SetErr(&bytes.Buffer{})
				command.SetArgs(args)
				if err := command.Execute(); err == nil {
					t.Fatal("invalid declaration accepted")
				}
				if requests != 1 {
					t.Fatalf("requests=%d", requests)
				}
			})
		}
	}
}

func TestTemplateSpecDeliveryBundledSchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(findRepoRoot(t), "docs/ir-spec/en/examples/valid/delivery-code-outputs.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, declaration string
		valid             bool
	}{
		{"omitted", "", true},
		{"valid", `{"schemaVersion":1,"items":[{"key":"result","label":"最终结果","stepId":"stp_code001","portId":"file"}]}`, true},
		{"empty", `{}`, false},
		{"null", `null`, false},
		{"empty-items", `{"schemaVersion":1,"items":[]}`, false},
		{"unknown-version", `{"schemaVersion":2,"items":[]}`, false},
		{"missing-port", `{"schemaVersion":1,"items":[{"key":"result","label":"Result","stepId":"stp_code001"}]}`, false},
		{"unknown-field", `{"schemaVersion":1,"items":[{"key":"result","label":"Result","stepId":"stp_code001","portId":"file","filename":"one.json"}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var spec map[string]json.RawMessage
			if err := json.Unmarshal(raw, &spec); err != nil {
				t.Fatal(err)
			}
			delete(spec, "deliveryOutputs")
			if tc.declaration != "" {
				spec["deliveryOutputs"] = json.RawMessage(tc.declaration)
			}
			data, err := json.Marshal(spec)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "spec.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := loadTemplateSpecFile(path); (err == nil) != tc.valid {
				t.Fatalf("valid=%t err=%v", tc.valid, err)
			}
		})
	}
}
