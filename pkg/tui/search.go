package tui

import (
	"fmt"
	"sort"
	"strings"
)

func styleFileLine(line string, lineIndex int, matchLines []int, activeMatchLine int) string {
	if lineIndex == activeMatchLine {
		return activeMatchStyle.Render(line)
	}
	if containsInt(matchLines, lineIndex) {
		return matchLineStyle.Render(line)
	}
	return line
}

func fileLinePrefix(lineIndex, lineNumberWidth int, matchLines []int, activeMatchLine int) string {
	marker := fileGutterMarker(lineIndex, matchLines, activeMatchLine)
	switch marker {
	case ">":
		marker = gutterActiveStyle.Render(marker)
	case "*":
		marker = gutterMatchStyle.Render(marker)
	}
	lineNumber := lineNumberStyle.Render(fmt.Sprintf("%*d ", lineNumberWidth, lineIndex+1))
	return marker + " " + lineNumber
}

func fileGutterMarker(lineIndex int, matchLines []int, activeMatchLine int) string {
	if lineIndex == activeMatchLine {
		return ">"
	}
	if containsInt(matchLines, lineIndex) {
		return "*"
	}
	return " "
}

func highlightSearchTerm(line, query string, active bool) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return line
	}

	lowerLine := strings.ToLower(line)
	lowerQuery := strings.ToLower(q)
	if !strings.Contains(lowerLine, lowerQuery) {
		return line
	}

	style := termMatchStyle
	if active {
		style = termActiveStyle
	}

	result := strings.Builder{}
	start := 0
	for {
		idx := strings.Index(strings.ToLower(line[start:]), lowerQuery)
		if idx < 0 {
			result.WriteString(line[start:])
			break
		}
		idx += start
		result.WriteString(line[start:idx])
		end := idx + len(q)
		if end > len(line) {
			end = len(line)
		}
		result.WriteString(style.Render(line[idx:end]))
		start = end
	}

	return result.String()
}

func visibleSegment(row string, colScroll, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(row)
	if len(r) == 0 {
		return ""
	}

	start := clamp(colScroll, 0, len(r))
	if start >= len(r) {
		return ""
	}
	end := minInt(len(r), start+width)
	out := string(r[start:end])
	if end < len(r) && width > 1 {
		out = truncate(out, width-1) + "~"
	}
	return out
}

func matchColumn(line, query string) int {
	q := strings.TrimSpace(query)
	if q == "" {
		return -1
	}

	lineRunes := []rune(line)
	lowerLineRunes := []rune(strings.ToLower(line))
	lowerQueryRunes := []rune(strings.ToLower(q))
	if len(lowerQueryRunes) == 0 || len(lowerLineRunes) < len(lowerQueryRunes) {
		return -1
	}

	for i := 0; i <= len(lowerLineRunes)-len(lowerQueryRunes); i++ {
		matched := true
		for j := 0; j < len(lowerQueryRunes); j++ {
			if lowerLineRunes[i+j] != lowerQueryRunes[j] {
				matched = false
				break
			}
		}
		if matched {
			return len(lineRunes[:i])
		}
	}

	return -1
}

func (m Model) keysForOutput() []string {
	keysSet := map[string]bool{}
	for pane := range m.selected {
		for key := range m.selected[pane] {
			keysSet[key] = true
		}
	}
	if len(keysSet) == 0 {
		if r := m.currentRow(); r != nil {
			keysSet[r.Key] = true
		}
	}
	keys := make([]string, 0, len(keysSet))
	for key := range keysSet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m *Model) applySearch() {
	if m.file == nil || m.file.Kind != FileYAML {
		return
	}
	query := strings.TrimSpace(strings.ToLower(m.file.SearchQuery))
	if query == "" {
		m.file.MatchLines = nil
		m.file.ActiveMatch = 0
		m.file.ActionStatus = ""
		return
	}

	matches := make([]int, 0)
	for i, line := range m.file.Lines {
		if strings.Contains(strings.ToLower(line), query) {
			matches = append(matches, i)
		}
	}
	m.file.MatchLines = matches
	m.file.ActiveMatch = 0
	if len(matches) == 0 {
		m.file.ActionStatus = "no matches"
		return
	}
	m.file.Scroll = matches[0]
	m.ensureActiveMatchVisible()
	m.file.ActionStatus = fmt.Sprintf("found %d matches", len(matches))
}

func (m *Model) ensureActiveMatchVisible() {
	if m.file == nil || len(m.file.MatchLines) == 0 {
		return
	}
	activeLine := m.file.activeMatchLine()
	if activeLine < 0 || activeLine >= len(m.file.Lines) {
		return
	}

	query := strings.TrimSpace(m.file.SearchQuery)
	if query == "" {
		return
	}

	matchCol := matchColumn(m.file.Lines[activeLine], query)
	if matchCol < 0 {
		return
	}

	contentWidth := m.fileContentWidth()
	if contentWidth <= 0 {
		return
	}

	matchStart := matchCol
	matchEnd := matchCol + len([]rune(query)) - 1
	viewStart := m.file.ColScroll
	viewEnd := viewStart + contentWidth - 1

	if matchStart < viewStart {
		m.file.ColScroll = matchStart
	} else if matchEnd > viewEnd {
		m.file.ColScroll = matchEnd - contentWidth + 1
	}
	m.file.ColScroll = clamp(m.file.ColScroll, 0, m.fileMaxColScroll())
}

func (m *Model) fileMaxColScroll() int {
	if m.file == nil || len(m.file.Lines) == 0 {
		return 0
	}
	contentWidth := m.fileContentWidth()
	longest := 0
	for _, line := range m.file.Lines {
		l := len([]rune(line))
		if l > longest {
			longest = l
		}
	}
	return maxInt(0, longest-contentWidth)
}

func (m *Model) fileContentWidth() int {
	if m.file == nil {
		return 1
	}
	lineNumberWidth := len(fmt.Sprintf("%d", maxInt(1, len(m.file.Lines))))
	return maxInt(1, m.width-lineNumberWidth-4)
}

func (m *View) activeMatchLine() int {
	if m == nil || len(m.MatchLines) == 0 {
		return -1
	}
	if m.ActiveMatch < 0 || m.ActiveMatch >= len(m.MatchLines) {
		return -1
	}
	return m.MatchLines[m.ActiveMatch]
}
