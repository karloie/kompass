package tui

import "testing"

func TestResolveChromaStyleReturnsFallbackForUnknownNames(t *testing.T) {
	style := resolveChromaStyle("not-a-style", "also-not-a-style")
	if style == nil {
		t.Fatalf("expected style fallback, got nil")
	}
}

func TestYAMLHighlightUsesConfiguredThemeStyle(t *testing.T) {
	if yamlChromaStyle == nil {
		t.Fatalf("expected yaml chroma style to be initialized")
	}
	line := "apiVersion: v1"
	out := highlightYAMLLine(line)
	if out == "" {
		t.Fatalf("expected highlighted output for yaml line")
	}
}

func TestApplyHighlightThemeUpdatesGlobalStyles(t *testing.T) {
	original := currentHighlightTheme
	t.Cleanup(func() { applyHighlightTheme(original) })

	custom := original
	custom.DescribeKeyColor = "45"
	custom.EventsKeyColor = "45"
	custom.YAMLStyleCandidates = []string{"native"}
	applyHighlightTheme(custom)

	if currentHighlightTheme.DescribeKeyColor != "45" {
		t.Fatalf("expected global theme to update describe key color, got %q", currentHighlightTheme.DescribeKeyColor)
	}
	if yamlChromaStyle == nil {
		t.Fatalf("expected yaml style to remain configured")
	}
}

func TestHighlightYAMLLine_SimpleModeStylesKey(t *testing.T) {
	original := currentHighlightTheme
	t.Cleanup(func() { applyHighlightTheme(original) })

	custom := original
	custom.YAMLSimpleMode = true
	applyHighlightTheme(custom)

	line := "metadata:"
	out := highlightYAMLLine(line)
	want := highlightSimpleYAMLLine(line)
	if out != want {
		t.Fatalf("expected highlightYAMLLine to follow simple mode path\nwant: %q\ngot:  %q", want, out)
	}
}

func TestHighlightYAMLLine_KeyOverrideModePath(t *testing.T) {
	original := currentHighlightTheme
	t.Cleanup(func() { applyHighlightTheme(original) })

	custom := original
	custom.YAMLSimpleMode = false
	applyHighlightTheme(custom)

	line := "name: app"
	out := highlightYAMLLine(line)
	want := highlightYAMLLineWithKeyOverride(line)
	if out != want {
		t.Fatalf("expected highlightYAMLLine to follow key override mode path\nwant: %q\ngot:  %q", want, out)
	}
}

func TestHighlightResourceLine_HubbleUsesHubbleHighlighter(t *testing.T) {
	line := `2 Mar 15 21:53:20.909: ns/web-123:51064 (ID:56723) -> ns/db-456:7687 (ID:26403) policy-verdict:L3-Only`
	got := highlightResourceLine("hubble", line)
	want := highlightHubbleLine(line)
	if got != want {
		t.Fatalf("expected hubble page dispatch to use highlightHubbleLine\nwant: %q\ngot:  %q", want, got)
	}
}

func TestHighlightResourceLine_HubbleLeavesPlainTextUntouched(t *testing.T) {
	line := "some plain message with no flow markers"
	out := highlightResourceLine("hubble", line)
	if out != line {
		t.Fatalf("expected plain hubble line to remain unchanged\nwant: %q\ngot:  %q", line, out)
	}
}
