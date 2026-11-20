package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRunValidatePreservesExactRowNumbers(t *testing.T) {
	const row = `{"seed":9007199254740993,"max":9223372036854775807,"min":-9223372036854775808,"width":1440,"scientific":1.44e3,"fraction":0.10000000000000001,"enabled":true,"text":"001","empty":null}`
	want := map[string]string{
		"Seed": "9007199254740993", "max": "9223372036854775807", "min": "-9223372036854775808",
		"width": "1440", "scientific": "1.44e3", "fraction": "0.10000000000000001",
		"enabled": "true", "text": "001", "empty": "",
	}
	for _, tc := range []struct {
		name, input string
		count       int
	}{
		{"json", "[" + row + "]", 1},
		{"jsonl", row + "\n\n" + row + "\n", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var received []map[string]string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/loom/v1/officialTemplates/integer-v2/schema":
					_, _ = w.Write([]byte(`{"columns":[{"fieldKey":"seed","headerLabel":"Seed"}]}`))
				case "/loom/v1/officialTemplates/integer-v2:validateRows":
					var body struct {
						Rows []map[string]string `json:"rows"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("decode request: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					received = body.Rows
					_, _ = w.Write([]byte(`{"valid":true,"rowErrors":[]}`))
				default:
					t.Errorf("unexpected path: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			inputPath := filepath.Join(t.TempDir(), "rows."+tc.name)
			if err := os.WriteFile(inputPath, []byte(tc.input), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := newRunValidateCmd(&rootOptions{server: server.URL + "/loom/v1", timeout: time.Second, output: "json"})
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetArgs([]string{"integer-v2", "--file", inputPath})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if len(received) != tc.count {
				t.Fatalf("received %d rows, want %d", len(received), tc.count)
			}
			for i, row := range received {
				if !reflect.DeepEqual(row, want) {
					t.Errorf("row %d = %#v, want %#v", i, row, want)
				}
			}
		})
	}
}

func TestLoadTemplateRowsRejectsMalformedAndNonScalarInput(t *testing.T) {
	for _, tc := range []struct {
		name, input, wantError string
	}{
		{"array trailing value", `[{}] {}`, "parse json array"},
		{"array trailing garbage", `[{}] garbage`, "parse json array"},
		{"jsonl trailing value", `{} {}`, "parse jsonl line 1"},
		{"jsonl trailing garbage", `{} garbage`, "parse jsonl line 1"},
		{"invalid second line", "{}\n{", "parse jsonl line 2"},
		{"fraction syntax", `[{"count":1.}]`, "parse json array"},
		{"nested object", `[{"count":{"value":2}}]`, "unsupported non-scalar"},
		{"nested array", `{"count":[2]}`, "unsupported non-scalar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputPath := filepath.Join(t.TempDir(), "rows.json")
			if err := os.WriteFile(inputPath, []byte(tc.input), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadTemplateRows(inputPath); err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
		})
	}
}
