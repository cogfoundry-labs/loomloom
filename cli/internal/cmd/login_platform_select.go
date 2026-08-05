package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/cogfoundry-labs/loomloom/cli/internal/platform"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var errLoginPlatformCancelled = errors.New("login platform selection cancelled")

var errRawModeUnavailable = errors.New("raw terminal mode unavailable")

type loginPlatformOption struct {
	id    platform.ID
	label string
}

func loginPlatformOptions() []loginPlatformOption {
	return []loginPlatformOption{
		{id: platform.ShengSuanYun, label: "胜算云 ShengSuanYun — 推荐中国大陆用户"},
		{id: platform.CogFoundry, label: "CogFoundry — Recommended for users in Singapore and other countries/regions"},
	}
}

func resolveLoginPlatform(cmd *cobra.Command, opts *rootOptions) (string, platform.Platform, error) {
	if server := strings.TrimSpace(opts.server); server != "" {
		return server, platform.InferFromServer(server), nil
	}

	if opts.output == "json" {
		return "", platform.Platform{}, loginPlatformSelectionRequiredError()
	}

	inFile, inIsFile := cmd.InOrStdin().(*os.File)
	outFile, outIsFile := cmd.ErrOrStderr().(*os.File)
	if !inIsFile || !outIsFile || !term.IsTerminal(int(inFile.Fd())) || !term.IsTerminal(int(outFile.Fd())) {
		return "", platform.Platform{}, loginPlatformSelectionRequiredError()
	}

	options := loginPlatformOptions()
	_, _ = fmt.Fprintln(outFile, "尚未指定 --server，请选择要登录的平台（↑/↓ 选择，Enter 确认，Ctrl+C 取消）：")
	idx, err := runArrowKeyMenu(inFile, outFile, options)
	if errors.Is(err, errRawModeUnavailable) {
		idx, err = promptNumberedChoice(inFile, outFile, options)
	}
	if err != nil {
		if errors.Is(err, errLoginPlatformCancelled) {
			return "", platform.Platform{}, fmt.Errorf("已取消登录：未选择平台")
		}
		return "", platform.Platform{}, fmt.Errorf("read platform selection: %w", err)
	}

	chosen, _ := platform.ByID(options[idx].id)
	return chosen.DefaultServer, chosen, nil
}

func loginPlatformSelectionRequiredError() error {
	servers := make([]string, 0, len(loginPlatformOptions()))
	for _, option := range loginPlatformOptions() {
		selected, ok := platform.ByID(option.id)
		if !ok {
			continue
		}
		servers = append(servers, fmt.Sprintf("%s: %s", selected.DisplayName, selected.DefaultServer))
	}
	return fmt.Errorf(
		"login platform selection requires an interactive terminal; pass --server <url> explicitly (%s), or select a saved profile with `loomloom server use <name>`",
		strings.Join(servers, "; "),
	)
}

type menuKey int

const (
	menuKeyNone menuKey = iota
	menuKeyUp
	menuKeyDown
	menuKeyEnter
	menuKeyCancel
)

func decodeMenuKey(reader *bufio.Reader) (menuKey, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return menuKeyNone, err
	}
	switch b {
	case '\r', '\n':
		return menuKeyEnter, nil
	case 3, 'q', 'Q':
		return menuKeyCancel, nil
	case 'k', 'K':
		return menuKeyUp, nil
	case 'j', 'J':
		return menuKeyDown, nil
	case 27:
		b2, err := reader.ReadByte()
		if err != nil {
			return menuKeyCancel, nil
		}
		if b2 != '[' {
			return menuKeyNone, nil
		}
		b3, err := reader.ReadByte()
		if err != nil {
			return menuKeyNone, err
		}
		switch b3 {
		case 'A':
			return menuKeyUp, nil
		case 'B':
			return menuKeyDown, nil
		default:
			return menuKeyNone, nil
		}
	default:
		return menuKeyNone, nil
	}
}

func runMenuLoop(reader *bufio.Reader, count int, render func(selected int, first bool)) (int, error) {
	selected := 0
	render(selected, true)
	for {
		key, err := decodeMenuKey(reader)
		if err != nil {
			return -1, err
		}
		switch key {
		case menuKeyUp:
			selected = (selected - 1 + count) % count
			render(selected, false)
		case menuKeyDown:
			selected = (selected + 1) % count
			render(selected, false)
		case menuKeyEnter:
			render(selected, false)
			return selected, nil
		case menuKeyCancel:
			return -1, errLoginPlatformCancelled
		}
	}
}

func runArrowKeyMenu(f *os.File, out io.Writer, options []loginPlatformOption) (int, error) {
	oldState, err := term.MakeRaw(int(f.Fd()))
	if err != nil {
		return -1, fmt.Errorf("%w: %v", errRawModeUnavailable, err)
	}
	defer func() { _ = term.Restore(int(f.Fd()), oldState) }()

	_, _ = fmt.Fprint(out, "\x1b[?25l")                    // hide cursor
	defer func() { _, _ = fmt.Fprint(out, "\x1b[?25h") }() // show cursor

	reader := bufio.NewReader(f)
	idx, err := runMenuLoop(reader, len(options), func(selected int, first bool) {
		renderPlatformMenu(out, options, selected, first)
	})
	_, _ = fmt.Fprint(out, "\r\n")
	return idx, err
}

func renderPlatformMenu(out io.Writer, options []loginPlatformOption, selected int, first bool) {
	if !first {
		_, _ = fmt.Fprintf(out, "\x1b[%dA", len(options))
	}
	for i, opt := range options {
		_, _ = fmt.Fprint(out, "\r\x1b[2K")
		if i == selected {
			_, _ = fmt.Fprintf(out, "\x1b[36m> %s\x1b[0m\r\n", opt.label)
		} else {
			_, _ = fmt.Fprintf(out, "  %s\r\n", opt.label)
		}
	}
}

func promptNumberedChoice(in io.Reader, out io.Writer, options []loginPlatformOption) (int, error) {
	for i, opt := range options {
		_, _ = fmt.Fprintf(out, "  %d. %s\n", i+1, opt.label)
	}
	_, _ = fmt.Fprintf(out, "请输入序号后回车（直接回车默认选择 1）：")

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return -1, err
		}
		return 0, nil
	}
	text := strings.TrimSpace(scanner.Text())
	if text == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 || n > len(options) {
		return -1, fmt.Errorf("无效的选择 %q；请输入 1 到 %d 之间的数字", text, len(options))
	}
	return n - 1, nil
}
