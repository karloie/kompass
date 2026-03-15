package tui

type tuiTheme struct {
	UI        uiTheme
	Highlight highlightTheme
}

var defaultTUITheme = tuiTheme{
	UI:        defaultUITheme,
	Highlight: defaultHighlightTheme,
}

var currentTUITheme = defaultTUITheme

func applyTUITheme(theme tuiTheme) {
	currentTUITheme = theme
	applyUITheme(theme.UI)
	applyHighlightTheme(theme.Highlight)
}

func init() {
	applyTUITheme(currentTUITheme)
}
