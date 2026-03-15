package tui

import (
	"os/exec"
	"strings"

	"github.com/karloie/kompass/pkg/tree"
)

func viewDescribe(r Row, context, defaultNamespace string) *View {
	args, _ := buildDescribeArgs(r, context, defaultNamespace)
	out, err := exec.Command("kubectl", args...).CombinedOutput()

	body := strings.TrimRight(string(out), "\n")
	if body == "" {
		body = "(no output)"
	}
	if err != nil {
		body = body + "\n\nerror: " + err.Error()
	}

	title := "kubectl " + strings.Join(args, " ")
	raw := body
	rows := strings.Split(raw, "\n")
	return &View{Kind: FileOutput, Title: title, Rows: rows, Raw: raw}
}

func buildDescribeArgs(r Row, context, defaultNamespace string) ([]string, string) {
	ref := tree.ParseResourceKeyRef(r.Key)
	resourceType := strings.TrimSpace(ref.Type)
	if resourceType == "" {
		resourceType = strings.TrimSpace(r.Type)
	}
	name := strings.TrimSpace(ref.Name)
	if name == "" {
		name = strings.TrimSpace(r.Name)
	}
	namespace := strings.TrimSpace(ref.Namespace)
	if namespace == "" {
		namespace = strings.TrimSpace(defaultNamespace)
	}

	args := make([]string, 0, 8)
	if strings.TrimSpace(context) != "" {
		args = append(args, "--context", strings.TrimSpace(context))
	}
	args = append(args, "describe", resourceType)
	if name != "" {
		args = append(args, name)
	}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	title := resourceType
	if name != "" {
		title += "/" + name
	}
	return args, title
}

func viewHelp() *View {
	rows := []string{
		"Rows",
		"  Up/Down or j/k  move Row",
		"  f               open row filter input",
		"  Tab/Shift+Tab   jump to next/previous root",
		"  1/2             switch to Tree/Single pane",
		"",
		"Actions",
		"  Space           toggle Row selection",
		"  Enter           run kubectl describe and open result",
		"  r               refresh selector data from cluster",
		"  o               output selected/current keys and quit",
		"  + / -           increase/decrease footer panel height",
		"",
		"File",
		"  Up/Down, PgUp/PgDn  scroll",
		"  g / G               go to top/bottom",
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
