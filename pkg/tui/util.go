package tui

import (
	"fmt"
	"strings"

	kube "github.com/karloie/kompass/pkg/kube"
	"go.yaml.in/yaml/v2"
)

func viewYaml(r Row, resource *kube.Resource) *View {
	var content any
	if resource != nil && resource.Resource != nil {
		content = resource.Resource
	} else {
		content = map[string]any{
			"key":      r.Key,
			"type":     r.Type,
			"name":     r.Name,
			"status":   r.Status,
			"metadata": r.Metadata,
		}
	}
	b, err := yaml.Marshal(content)
	if err != nil {
		b = []byte("error: failed to render yaml")
	}
	raw := strings.TrimRight(string(b), "\n")
	rows := strings.Split(raw, "\n")
	return &View{Kind: FileYAML, Title: fmt.Sprintf("%s/%s", r.Type, r.Name), Rows: rows, Raw: raw}
}

func viewHelp() *View {
	rows := []string{
		"Rows",
		"  Up/Down or j/k  move Row",
		"  Tab             next pane",
		"  Shift+Tab       previous pane",
		"  1/2             jump to Tree/Single",
		"",
		"Actions",
		"  Space           toggle Row selection",
		"  Enter           open YAML file for current Row",
		"  o               output selected/current keys and quit",
		"  + / -           increase/decrease footer panel height",
		"",
		"File",
		"  Up/Down, PgUp/PgDn  scroll",
		"  g / G               jump to top/bottom",
		"  Left/Right or h/l   pan long lines",
		"  Home/End            pan to line start/end",
		"  /                    start search",
		"  Ctrl+U               clear search query",
		"  n / N                next/previous match",
		"  y                    copy file content",
		"  e                    open in $EDITOR (read-only where supported)",
		"  Esc or q            close file",
		"",
		"Exit",
		"  Esc / Ctrl+C     quit application",
	}
	raw := strings.Join(rows, "\n")
	return &View{Kind: FileHelp, Title: "Keybindings", Rows: rows, Raw: raw}
}

func fit(s string, width int) string {
	if width <= 0 {
		return s
	}
	return truncate(s, width)
}

func hasArg(args []string, needle string) bool {
	for _, arg := range args {
		if arg == needle {
			return true
		}
	}
	return false
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return string(r[:1])
	}
	if width <= 3 {
		return string(r[:width])
	}
	return string(r[:width-3]) + "..."
}

func pad(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) >= width {
		return string(r[:width])
	}
	return s + strings.Repeat(" ", width-len(r))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func containsInt(values []int, needle int) bool {
	for _, v := range values {
		if v == needle {
			return true
		}
	}
	return false
}
