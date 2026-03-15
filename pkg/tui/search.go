package tui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type queryTerm struct {
	negated  bool
	wildcard bool
	re       *regexp.Regexp
	lower    string
}

type queryMatcher struct {
	terms []queryTerm
}

func rowStyle(row string, rowIndex int, matchRows []int, activeMatchRow int) string {
	if rowIndex == activeMatchRow {
		return activeMatchStyle.Render(row)
	}
	if containsInt(matchRows, rowIndex) {
		return matchRowStyle.Render(row)
	}
	return row
}

func rowPrefix(rowIndex, rowNumberWidth int, matchRows []int, activeMatchRow int) string {
	marker := fileGutterMarker(rowIndex, matchRows, activeMatchRow)
	switch marker {
	case ">":
		marker = gutterActiveStyle.Render(marker)
	case "*":
		marker = gutterMatchStyle.Render(marker)
	}
	rowNumber := rowNumberStyle.Render(fmt.Sprintf("%*d ", rowNumberWidth, rowIndex+1))
	return marker + " " + rowNumber
}

func fileGutterMarker(rowIndex int, matchRows []int, activeMatchRow int) string {
	if rowIndex == activeMatchRow {
		return ">"
	}
	if containsInt(matchRows, rowIndex) {
		return "*"
	}
	return " "
}

func highlightSearchTerm(row, query string, active bool) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return row
	}

	lowerRow := strings.ToLower(row)
	lowerQuery := strings.ToLower(q)
	if !strings.Contains(lowerRow, lowerQuery) {
		return row
	}

	style := termMatchStyle
	if active {
		style = termActiveStyle
	}

	result := strings.Builder{}
	start := 0
	for {
		idx := strings.Index(strings.ToLower(row[start:]), lowerQuery)
		if idx < 0 {
			result.WriteString(row[start:])
			break
		}
		idx += start
		result.WriteString(row[start:idx])
		end := idx + len(q)
		if end > len(row) {
			end = len(row)
		}
		result.WriteString(style.Render(row[idx:end]))
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

func matchColumn(row, query string) int {
	q := strings.TrimSpace(query)
	if q == "" {
		return -1
	}

	rowRunes := []rune(row)
	lowerRowRunes := []rune(strings.ToLower(row))
	lowerQueryRunes := []rune(strings.ToLower(q))
	if len(lowerQueryRunes) == 0 || len(lowerRowRunes) < len(lowerQueryRunes) {
		return -1
	}

	for i := 0; i <= len(lowerRowRunes)-len(lowerQueryRunes); i++ {
		matched := true
		for j := 0; j < len(lowerQueryRunes); j++ {
			if lowerRowRunes[i+j] != lowerQueryRunes[j] {
				matched = false
				break
			}
		}
		if matched {
			return len(rowRunes[:i])
		}
	}

	return -1
}

func buildQueryMatcher(raw string) queryMatcher {
	items := strings.Fields(strings.TrimSpace(raw))
	terms := make([]queryTerm, 0, len(items))

	for _, item := range items {
		negated := false
		token := item
		if strings.HasPrefix(token, "!") {
			negated = true
			token = strings.TrimSpace(token[1:])
		}
		if token == "" {
			continue
		}

		wildcard := strings.Contains(token, "*") || strings.Contains(token, "?")
		var re *regexp.Regexp
		if wildcard {
			re = globToRegexp(token)
		}

		terms = append(terms, queryTerm{
			negated:  negated,
			wildcard: wildcard,
			re:       re,
			lower:    strings.ToLower(token),
		})
	}

	return queryMatcher{terms: terms}
}

func (m queryMatcher) test(value string) bool {
	if len(m.terms) == 0 {
		return true
	}

	lower := strings.ToLower(value)
	hasPositive := false

	for _, term := range m.terms {
		if term.negated {
			if term.wildcard {
				if term.re != nil && term.re.MatchString(value) {
					return false
				}
			} else if strings.Contains(lower, term.lower) {
				return false
			}
			continue
		}

		hasPositive = true
		if term.wildcard {
			if term.re == nil || !term.re.MatchString(value) {
				return false
			}
			continue
		}
		if !strings.Contains(lower, term.lower) {
			return false
		}
	}

	return hasPositive || len(m.terms) > 0
}

func globToRegexp(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("(?i)")
	for _, ch := range pattern {
		switch ch {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	re, err := regexp.Compile(b.String())
	if err != nil {
		return regexp.MustCompile("^$")
	}
	return re
}

func rowSearchText(row Row) string {
	parts := []string{row.Type, row.Name, row.Key, row.Status, row.Text, row.Plain, row.PlainText}
	for k, v := range row.Metadata {
		parts = append(parts, k, fmt.Sprint(v))
	}
	return strings.Join(parts, " ")
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
	if m.view == nil || m.view.Kind != FileOutput {
		return
	}
	query := strings.TrimSpace(strings.ToLower(m.view.SearchQuery))
	if query == "" {
		m.view.MatchRows = nil
		m.view.ActiveMatch = 0
		m.view.ActionStatus = ""
		return
	}

	matches := make([]int, 0)
	for i, row := range m.view.Rows {
		if strings.Contains(strings.ToLower(row), query) {
			matches = append(matches, i)
		}
	}
	m.view.MatchRows = matches
	m.view.ActiveMatch = 0
	if len(matches) == 0 {
		m.view.ActionStatus = "no matches"
		return
	}
	m.view.Scroll = matches[0]
	m.ensureActiveMatchVisible()
	m.view.ActionStatus = fmt.Sprintf("found %d matches", len(matches))
}

func (m *Model) ensureActiveMatchVisible() {
	if m.view == nil || len(m.view.MatchRows) == 0 {
		return
	}
	activeRow := m.view.activeMatchRow()
	if activeRow < 0 || activeRow >= len(m.view.Rows) {
		return
	}

	query := strings.TrimSpace(m.view.SearchQuery)
	if query == "" {
		return
	}

	matchCol := matchColumn(m.view.Rows[activeRow], query)
	if matchCol < 0 {
		return
	}

	contentWidth := m.contentWidth()
	if contentWidth <= 0 {
		return
	}

	matchStart := matchCol
	matchEnd := matchCol + len([]rune(query)) - 1
	viewStart := m.view.ColScroll
	viewEnd := viewStart + contentWidth - 1

	if matchStart < viewStart {
		m.view.ColScroll = matchStart
	} else if matchEnd > viewEnd {
		m.view.ColScroll = matchEnd - contentWidth + 1
	}
	m.view.ColScroll = clamp(m.view.ColScroll, 0, m.maxColScroll())
}

func (m *Model) maxColScroll() int {
	if m.view == nil || len(m.view.Rows) == 0 {
		return 0
	}
	contentWidth := m.contentWidth()
	longest := 0
	for _, row := range m.view.Rows {
		l := len([]rune(row))
		if l > longest {
			longest = l
		}
	}
	return maxInt(0, longest-contentWidth)
}

func (m *Model) contentWidth() int {
	if m.view == nil {
		return 1
	}
	rowNumberWidth := len(fmt.Sprintf("%d", maxInt(1, len(m.view.Rows))))
	return maxInt(1, m.width-rowNumberWidth-4)
}

func (m *View) activeMatchRow() int {
	if m == nil || len(m.MatchRows) == 0 {
		return -1
	}
	if m.ActiveMatch < 0 || m.ActiveMatch >= len(m.MatchRows) {
		return -1
	}
	return m.MatchRows[m.ActiveMatch]
}
