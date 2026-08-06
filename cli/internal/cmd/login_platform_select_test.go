package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cogfoundry-labs/loomloom/cli/internal/platform"
)

func TestDecodeMenuKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  menuKey
	}{
		{"enter cr", "\r", menuKeyEnter},
		{"enter lf", "\n", menuKeyEnter},
		{"ctrl-c", "\x03", menuKeyCancel},
		{"q cancels", "q", menuKeyCancel},
		{"vim up", "k", menuKeyUp},
		{"vim down", "j", menuKeyDown},
		{"arrow up", "\x1b[A", menuKeyUp},
		{"arrow down", "\x1b[B", menuKeyDown},
		{"unrelated escape sequence", "\x1b[C", menuKeyNone},
		{"unrelated key", "x", menuKeyNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			got, err := decodeMenuKey(reader)
			if err != nil {
				t.Fatalf("decodeMenuKey(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("decodeMenuKey(%q) = %v want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDecodeMenuKeyBareEscapeCancels(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\x1b"))
	got, err := decodeMenuKey(reader)
	if err != nil {
		t.Fatalf("decodeMenuKey error = %v", err)
	}
	if got != menuKeyCancel {
		t.Fatalf("decodeMenuKey(bare ESC) = %v want menuKeyCancel", got)
	}
}

func TestRunMenuLoopArrowNavigationAndEnter(t *testing.T) {
	// Down, down (wraps back to option 0 with 2 options), up, then Enter.
	reader := bufio.NewReader(strings.NewReader("\x1b[B\x1b[B\x1b[A\r"))
	var renders []int
	idx, err := runMenuLoop(reader, 2, func(selected int, first bool) {
		renders = append(renders, selected)
	})
	if err != nil {
		t.Fatalf("runMenuLoop error = %v", err)
	}
	if idx != 1 {
		t.Fatalf("selected index = %d want 1", idx)
	}
	want := []int{0, 1, 0, 1, 1}
	if len(renders) != len(want) {
		t.Fatalf("renders = %v want %v", renders, want)
	}
	for i := range want {
		if renders[i] != want[i] {
			t.Fatalf("renders = %v want %v", renders, want)
		}
	}
}

func TestRunMenuLoopCancel(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\x1b[B\x03"))
	_, err := runMenuLoop(reader, 2, func(selected int, first bool) {})
	if !errors.Is(err, errLoginPlatformCancelled) {
		t.Fatalf("error = %v want errLoginPlatformCancelled", err)
	}
}

func TestRunMenuLoopEOFPropagatesError(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(""))
	_, err := runMenuLoop(reader, 2, func(selected int, first bool) {})
	if !errors.Is(err, io.EOF) {
		t.Fatalf("error = %v want io.EOF", err)
	}
}

func TestPromptNumberedChoiceValidSelection(t *testing.T) {
	options := loginPlatformOptions()
	var out bytes.Buffer
	idx, err := promptNumberedChoice(strings.NewReader("2\n"), &out, options)
	if err != nil {
		t.Fatalf("promptNumberedChoice error = %v", err)
	}
	if idx != 1 {
		t.Fatalf("idx = %d want 1", idx)
	}
	if !strings.Contains(out.String(), "1. ") || !strings.Contains(out.String(), "2. ") {
		t.Fatalf("prompt output missing numbered options:\n%s", out.String())
	}
}

func TestPromptNumberedChoiceDefaultsToFirstOnEmptyInput(t *testing.T) {
	options := loginPlatformOptions()
	var out bytes.Buffer
	idx, err := promptNumberedChoice(strings.NewReader("\n"), &out, options)
	if err != nil {
		t.Fatalf("promptNumberedChoice error = %v", err)
	}
	if idx != 0 {
		t.Fatalf("idx = %d want 0", idx)
	}
}

func TestPromptNumberedChoiceDefaultsToFirstOnEOF(t *testing.T) {
	options := loginPlatformOptions()
	var out bytes.Buffer
	idx, err := promptNumberedChoice(strings.NewReader(""), &out, options)
	if err != nil {
		t.Fatalf("promptNumberedChoice error = %v", err)
	}
	if idx != 0 {
		t.Fatalf("idx = %d want 0", idx)
	}
}

func TestPromptNumberedChoiceRejectsInvalidInput(t *testing.T) {
	options := loginPlatformOptions()
	var out bytes.Buffer
	for _, input := range []string{"0\n", "3\n", "abc\n"} {
		if _, err := promptNumberedChoice(strings.NewReader(input), &out, options); err == nil {
			t.Fatalf("promptNumberedChoice(%q) expected error", input)
		}
	}
}

func TestResolveLoginPlatformUsesExplicitServer(t *testing.T) {
	isolateCmdConfigHome(t)
	opts := &rootOptions{server: "https://loomloom.cogfoundry.ai/loom/v1", output: "text"}
	cmd := NewRootCmd()
	cmd.SetIn(&bytes.Buffer{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	server, resolved, err := resolveLoginPlatform(cmd, opts)
	if err != nil {
		t.Fatalf("resolveLoginPlatform error = %v", err)
	}
	if server != opts.server {
		t.Fatalf("server = %q want %q", server, opts.server)
	}
	if resolved.ID != platform.CogFoundry {
		t.Fatalf("platform = %q want %q", resolved.ID, platform.CogFoundry)
	}
}

func TestResolveLoginPlatformRequiresExplicitServerWhenNonInteractive(t *testing.T) {
	isolateCmdConfigHome(t)
	opts := &rootOptions{server: "", output: "text"}
	cmd := NewRootCmd()
	// bytes.Buffer is not an *os.File, so an interactive platform selection
	// cannot be performed.
	cmd.SetIn(&bytes.Buffer{})
	var errOutput bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&errOutput)

	server, resolved, err := resolveLoginPlatform(cmd, opts)
	if err == nil {
		t.Fatal("resolveLoginPlatform expected an explicit platform selection error")
	}
	if server != "" || resolved != (platform.Platform{}) {
		t.Fatalf("server = %q platform = %+v want empty results", server, resolved)
	}
	for _, want := range []string{
		"requires an interactive terminal",
		"pass --server <url> explicitly",
		"https://loomloom.shengsuanyun.com/loom/v1",
		"https://loomloom.cogfoundry.ai/loom/v1",
		"loomloom server use <name>",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q want substring %q", err, want)
		}
	}
	if errOutput.Len() != 0 {
		t.Fatalf("expected no interactive prompt output in non-interactive mode, got:\n%s", errOutput.String())
	}
}

func TestResolveLoginPlatformRequiresExplicitServerForJSONOutput(t *testing.T) {
	isolateCmdConfigHome(t)
	opts := &rootOptions{server: "", output: "json"}
	cmd := NewRootCmd()
	cmd.SetIn(&bytes.Buffer{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	server, resolved, err := resolveLoginPlatform(cmd, opts)
	if err == nil {
		t.Fatal("resolveLoginPlatform expected an explicit platform selection error")
	}
	if server != "" || resolved != (platform.Platform{}) {
		t.Fatalf("server = %q platform = %+v want empty results", server, resolved)
	}
	if !strings.Contains(err.Error(), "pass --server <url> explicitly") {
		t.Fatalf("error = %q want actionable --server guidance", err)
	}
}
