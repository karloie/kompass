package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func formatSelectionOutput(keys []string, outputJSON bool) (string, error) {
	if len(keys) == 0 {
		return "", nil
	}
	if outputJSON {
		b, err := json.Marshal(keys)
		if err != nil {
			return "", err
		}
		return string(b) + "\n", nil
	}
	return strings.Join(keys, "\n") + "\n", nil
}

func copyToClipboard(content string) error {
	candidates := [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"pbcopy"}}
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err != nil {
			continue
		}
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Stdin = strings.NewReader(content)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no supported clipboard tool found (wl-copy/xclip/pbcopy)")
}

func openInEditorCmd(content string) tea.Cmd {
	return func() tea.Msg {
		tmp, err := os.CreateTemp("", "kompass-yaml-*.yaml")
		if err != nil {
			return editorDoneMsg{err: err}
		}
		defer os.Remove(tmp.Name())

		if _, err := tmp.WriteString(content); err != nil {
			_ = tmp.Close()
			return editorDoneMsg{err: err}
		}
		_ = tmp.Close()

		editorCmd, editorArgs := resolveEditorCommand(os.Getenv("EDITOR"), tmp.Name())
		cmd := exec.Command(editorCmd, editorArgs...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return editorDoneMsg{err: cmd.Run()}
	}
}

func resolveEditorCommand(editorEnv, filePath string) (string, []string) {
	editor := strings.TrimSpace(editorEnv)
	if editor == "" {
		editor = "vi"
	}

	parts := strings.Fields(editor)
	bin := parts[0]
	args := append([]string{}, parts[1:]...)
	base := strings.ToLower(filepath.Base(bin))

	// Open known editors in read-only/viewer mode to avoid accidental mutations.
	switch base {
	case "vi", "vim", "nvim", "view", "vim.basic", "gvim":
		if !hasArg(args, "-R") {
			args = append(args, "-R")
		}
	case "nano", "pico":
		if !hasArg(args, "-v") {
			args = append(args, "-v")
		}
	case "code", "code-insiders", "cursor", "windsurf":
		if !hasArg(args, "--wait") {
			args = append(args, "--wait")
		}
		if !hasArg(args, "--readonly") {
			args = append(args, "--readonly")
		}
	}

	args = append(args, filePath)
	return bin, args
}
