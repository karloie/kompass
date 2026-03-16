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

func rowPrefix(rowIndex, rowNumberWidth int) string {
	rowNumber := rowNumberStyle.Render(fmt.Sprintf("%*d ", rowNumberWidth, rowIndex+1))
	return "  " + rowNumber
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
	if strings.TrimSpace(row.SearchText) != "" {
		return row.SearchText
	}

	return buildRowSearchText(row)
}

func buildRowSearchText(row Row) string {
	parts := []string{row.Type, row.Name, row.Key, row.Status, row.Text, row.Plain, row.PlainText}
	for k, v := range row.Metadata {
		if k == "orphaned" {
			parts = append(parts, "single", fmt.Sprint(v))
			continue
		}
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

func (m *Model) maxColScroll() int {
	if m.view == nil {
		return 0
	}
	return maxColScrollForRows(m.view.Rows, m.contentWidth())
}

func (m *Model) contentWidth() int {
	if m.view == nil {
		return 1
	}
	rowNumberWidth := len(fmt.Sprintf("%d", maxInt(1, len(m.view.Rows))))
	return maxInt(1, m.width-rowNumberWidth-4)
}

func maxColScrollForRows(rows []string, width int) int {
	if len(rows) == 0 {
		return 0
	}
	longest := 0
	for _, row := range rows {
		l := len([]rune(row))
		if l > longest {
			longest = l
		}
	}
	return maxInt(0, longest-maxInt(1, width))
}

func (m *Model) maxMainColScroll() int {
	return m.maxMainColScrollForPane(m.activePane)
}

func (m *Model) maxMainColScrollForPane(pane int) int {
	if pane < 0 || pane >= len(m.rowsByPane) {
		return 0
	}
	rows := m.rowsByPane[pane]
	if len(rows) == 0 {
		return 0
	}

	cursor := 0
	if pane >= 0 && pane < len(m.cursorByPane) {
		cursor = clamp(m.cursorByPane[pane], 0, len(rows)-1)
	}

	lineRows := make([]string, 0, len(rows))
	for i, row := range rows {
		if row.Separator {
			continue
		}
		lineRows = append(lineRows, m.rowScrollableContentForPane(row, pane, i == cursor))
	}

	return maxColScrollForRows(lineRows, m.width)
}
