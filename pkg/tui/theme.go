package tui

import "strings"

type tuiTheme struct {
	UI        uiTheme
	Highlight highlightTheme
}

const defaultThemeName = "blue"

var (
	themePresetOrder = []string{"blue", "mint", "amber"}
	themePresets     map[string]tuiTheme
)

var defaultTUITheme = buildThemePresets()[defaultThemeName]

var currentTUITheme = defaultTUITheme
var currentThemeName = defaultThemeName

func buildThemePresets() map[string]tuiTheme {
	blue := tuiTheme{UI: defaultUITheme, Highlight: defaultHighlightTheme}

	mintUI := defaultUITheme
	mintUI.AccentBackground = "23"
	mintUI.ActiveTabBackground = "159"
	mintUI.CommandBarBackground = "238"
	mintUI.FocusedRowBackground = "30"
	mintUI.ContinuationForeground = "122"

	mintHighlight := defaultHighlightTheme
	mintHighlight.YAMLStyleCandidates = []string{"dracula", "native"}
	mintHighlight.YAMLKeyColor = "86"
	mintHighlight.DescribeKeyColor = "86"
	mintHighlight.EventsKeyColor = "86"
	mintHighlight.EventWarnColor = "151"
	mintHighlight.EventOKColor = "86"
	mintHighlight.LogLevelInfoColor = "86"
	mintHighlight.LogLevelWarnColor = "151"
	mintHighlight.LogKeyColor = "122"

	amberUI := defaultUITheme
	amberUI.AccentBackground = "94"
	amberUI.ActiveTabBackground = "222"
	amberUI.CommandBarBackground = "240"
	amberUI.FocusedRowBackground = "130"
	amberUI.ContinuationForeground = "179"

	amberHighlight := defaultHighlightTheme
	amberHighlight.YAMLStyleCandidates = []string{"solarized-dark", "native"}
	amberHighlight.YAMLKeyColor = "179"
	amberHighlight.DescribeKeyColor = "179"
	amberHighlight.EventsKeyColor = "179"
	amberHighlight.EventWarnColor = "215"
	amberHighlight.EventOKColor = "179"
	amberHighlight.LogLevelInfoColor = "179"
	amberHighlight.LogLevelWarnColor = "215"
	amberHighlight.LogKeyColor = "180"

	return map[string]tuiTheme{
		"blue":  blue,
		"mint":  {UI: mintUI, Highlight: mintHighlight},
		"amber": {UI: amberUI, Highlight: amberHighlight},
	}
}

func applyTUITheme(theme tuiTheme) {
	currentTUITheme = theme
	applyUITheme(theme.UI)
	applyHighlightTheme(theme.Highlight)
}

func setThemeByName(name string) bool {
	key := strings.ToLower(strings.TrimSpace(name))
	theme, ok := themePresets[key]
	if !ok {
		return false
	}
	currentThemeName = key
	applyTUITheme(theme)
	return true
}

func cycleTheme() string {
	idx := 0
	for i, name := range themePresetOrder {
		if name == currentThemeName {
			idx = i
			break
		}
	}
	next := themePresetOrder[(idx+1)%len(themePresetOrder)]
	_ = setThemeByName(next)
	return next
}

func init() {
	themePresets = buildThemePresets()
	_ = setThemeByName(defaultThemeName)
}
