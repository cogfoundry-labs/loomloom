package viewerassets

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedViewerIsCompleteAndContainsNoRemoteConfiguration(t *testing.T) {
	files, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	html, err := fs.ReadFile(files, "viewer.html")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(html), "runtime-config") || !strings.Contains(string(html), "assets/") {
		t.Fatal("not a standalone viewer")
	}
}
