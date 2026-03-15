package tui

import "testing"

func TestApplyTUIThemeUpdatesUIAndHighlightTheme(t *testing.T) {
	original := currentTUITheme
	t.Cleanup(func() { applyTUITheme(original) })

	next := original
	next.UI.AccentBackground = "25"
	next.Highlight.LogKeyColor = "45"
	next.Highlight.YAMLStyleCandidates = []string{"native"}

	applyTUITheme(next)

	if currentTUITheme.UI.AccentBackground != "25" {
		t.Fatalf("expected UI theme accent background update, got %q", currentTUITheme.UI.AccentBackground)
	}
	if currentUITheme.AccentBackground != "25" {
		t.Fatalf("expected active UI theme accent background update, got %q", currentUITheme.AccentBackground)
	}
	if currentHighlightTheme.LogKeyColor != "45" {
		t.Fatalf("expected highlight theme log key color update, got %q", currentHighlightTheme.LogKeyColor)
	}
	if yamlChromaStyle == nil {
		t.Fatalf("expected YAML style to remain configured after theme apply")
	}
}

func TestSetThemeByNameAndCycleTheme(t *testing.T) {
	originalName := currentThemeName
	t.Cleanup(func() { _ = setThemeByName(originalName) })

	if !setThemeByName("mint") {
		t.Fatalf("expected mint theme to be selectable")
	}
	if currentThemeName != "mint" {
		t.Fatalf("expected current theme mint, got %q", currentThemeName)
	}

	next := cycleTheme()
	if next != "amber" {
		t.Fatalf("expected cycle after mint to select amber, got %q", next)
	}
	if currentThemeName != "amber" {
		t.Fatalf("expected active theme amber after cycle, got %q", currentThemeName)
	}

	if setThemeByName("nope") {
		t.Fatalf("expected unknown theme name to be rejected")
	}
}
