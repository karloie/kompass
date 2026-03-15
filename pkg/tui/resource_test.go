package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderSelectionListOverlayKeepsLineWidth(t *testing.T) {
	contentWidth := 40
	contentLines := []string{
		strings.Repeat("A", contentWidth),
		strings.Repeat("B", contentWidth),
		strings.Repeat("C", contentWidth),
		strings.Repeat("D", contentWidth),
		strings.Repeat("E", contentWidth),
		strings.Repeat("F", contentWidth),
		strings.Repeat("G", contentWidth),
	}

	overlay := renderSelectionListOverlay(strings.Join(contentLines, "\n"), contentWidth, "Context", []string{"ctx-a", "ctx-b"}, 0)
	for _, line := range strings.Split(overlay, "\n") {
		plain := ansiEscapePattern.ReplaceAllString(line, "")
		if got := lipgloss.Width(plain); got != contentWidth {
			t.Fatalf("expected overlay line width %d, got %d for %q", contentWidth, got, plain)
		}
	}
}

func TestRenderModalOverlayMasksUnderlyingLineContent(t *testing.T) {
	content := strings.Join([]string{
		"0123456789012345678901234567890123456789",
		"left-side text that should be covered cleanly",
		"another long line with colored content beneath",
		"tail text should not poke through the box area",
		"0123456789012345678901234567890123456789",
	}, "\n")

	overlay := renderModalOverlay(content, 40,
		modalLine{text: "Context", style: modalTitleStyle},
		modalLine{text: "> ctx-a", style: modalOptionActiveStyle},
		modalLine{text: "  ctx-b", style: modalOptionDefaultStyle},
	)

	lines := strings.Split(overlay, "\n")
	for _, idx := range []int{1, 2, 3} {
		plain := ansiEscapePattern.ReplaceAllString(lines[idx], "")
		if got := lipgloss.Width(plain); got != 40 {
			t.Fatalf("expected masked line width 40, got %d for %q", got, plain)
		}
	}
}

func TestRenderModalOverlayPreservesAnsiOutsideMask(t *testing.T) {
	baseLine := "\x1b[36mnamespace=app\x1b[0m" + strings.Repeat(" ", 12) + "\x1b[33mready=1\x1b[0m"
	content := strings.Join([]string{
		strings.Repeat("-", 40),
		baseLine,
		strings.Repeat("-", 40),
	}, "\n")

	overlay := renderModalOverlay(content, 40,
		modalLine{text: "Context", style: modalTitleStyle},
	)

	line := strings.Split(overlay, "\n")[1]
	if !strings.Contains(line, "namespace=app") {
		t.Fatalf("expected left-side metadata text to remain visible, got %q", line)
	}
	if !strings.Contains(line, "ready=1") {
		t.Fatalf("expected right-side metadata text to remain visible, got %q", line)
	}
	if !strings.Contains(line, "\x1b[36m") || !strings.Contains(line, "\x1b[33m") {
		t.Fatalf("expected ANSI styling outside modal mask to be preserved, got %q", line)
	}
}

func TestRenderSelectionListOverlayShowsScrollingWindow(t *testing.T) {
	content := strings.Join([]string{
		strings.Repeat("-", 40),
		strings.Repeat("-", 40),
		strings.Repeat("-", 40),
		strings.Repeat("-", 40),
		strings.Repeat("-", 40),
		strings.Repeat("-", 40),
		strings.Repeat("-", 40),
	}, "\n")
	options := []string{"opt-00", "opt-01", "opt-02", "opt-03", "opt-04", "opt-05", "opt-06", "opt-07", "opt-08", "opt-09"}

	overlay := renderSelectionListOverlay(content, 40, "Namespace", options, 8)
	plain := ansiEscapePattern.ReplaceAllString(overlay, "")

	if !strings.Contains(plain, "> opt-08") {
		t.Fatalf("expected active option to remain visible, got %q", plain)
	}
	if strings.Contains(plain, "opt-00") {
		t.Fatalf("expected early off-screen options to be clipped, got %q", plain)
	}
	if !strings.Contains(plain, "opt-09") {
		t.Fatalf("expected later visible options to render, got %q", plain)
	}
	if !strings.Contains(plain, "Up/Down select") {
		t.Fatalf("expected hint line to remain visible, got %q", plain)
	}
}

func TestRenderSelectionListOverlayCollapsesForShortHeights(t *testing.T) {
	content := strings.Join([]string{
		strings.Repeat("-", 30),
		strings.Repeat("-", 30),
	}, "\n")

	overlay := renderSelectionListOverlay(content, 30, "Context", []string{"ctx-a", "ctx-b", "ctx-c"}, 1)
	lines := strings.Split(overlay, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected overlay to keep screen line count 2, got %d", len(lines))
	}
	plain := ansiEscapePattern.ReplaceAllString(overlay, "")
	if !strings.Contains(plain, "Context") {
		t.Fatalf("expected title to remain visible, got %q", plain)
	}
	if !strings.Contains(plain, "> ctx-b") {
		t.Fatalf("expected selected option to be visible in short modal, got %q", plain)
	}
	if strings.Contains(plain, "Enter apply") {
		t.Fatalf("expected hint to be omitted when height is too short, got %q", plain)
	}
}
