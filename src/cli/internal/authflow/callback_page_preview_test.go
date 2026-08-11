package authflow

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewCallbackPagesToDisk(t *testing.T) {
	dir := os.Getenv("PREVIEW_DIR")
	if dir == "" {
		t.Skip("set PREVIEW_DIR to write preview HTML files")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		v    CallbackPageVariant
		ok   bool
	}{
		{"generic-success", CallbackPageGeneric, true},
		{"generic-failure", CallbackPageGeneric, false},
		{"shengsuanyun-success", CallbackPageShengSuanYun, true},
		{"shengsuanyun-failure", CallbackPageShengSuanYun, false},
		{"cogfoundry-success", CallbackPageCogFoundry, true},
		{"cogfoundry-failure", CallbackPageCogFoundry, false},
	}

	for _, tc := range cases {
		w := httptest.NewRecorder()
		writeCallbackPage(w, tc.v, tc.ok)
		path := filepath.Join(dir, tc.name+".html")
		if err := os.WriteFile(path, w.Body.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}
