package tui

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}

	if m.view != nil {
		return m.toString()
	}

	header := m.Header()
	footer := m.Footer()
	rowsHeight := maxInt(1, m.height-1-m.footerHeight)
	parts := []string{header}
	rows := m.ViewRows(rowsHeight)
	parts = append(parts, rows, footer)
	out := strings.Join(parts, "\n")
	switch m.submode {
	case SubmodeConfirmQuit:
		return renderQuitConfirmOverlay(out, m.width)
	case SubmodeContextList:
		return renderSelectionListOverlay(out, m.width, "Context", m.contextList)
	case SubmodeNamespaceList:
		return renderSelectionListOverlay(out, m.width, "Namespace", m.namespaceList)
	case SubmodeFilter:
		return renderFilterOverlay(out, m.width, m.filterQuery)
	}
	return out
}

func renderQuitConfirmOverlay(content string, width int) string {
	return renderModalOverlay(content, width,
		modalLine{text: "Quit kompass?", style: modalTitleStyle},
		modalLine{text: "Enter confirm | Esc cancel", style: modalHintStyle},
	)
}

func renderFilterOverlay(content string, width int, query string) string {
	return renderModalOverlay(content, width,
		modalLine{text: "Filter", style: modalTitleStyle},
		modalLine{text: query + "_", style: modalBodyStyle},
		modalLine{text: "Enter apply | Esc cancel | Ctrl+U clear | Ctrl+L reset", style: modalHintStyle},
	)
}

func renderSelectionListOverlay(content string, width int, title string, state listPickerState) string {
	if state.Loading {
		return renderModalOverlay(content, width,
			modalLine{text: title, style: modalTitleStyle},
			modalLine{text: "Loading\u2026", style: modalBodyStyle},
		)
	}
	availableLines := maxInt(1, len(strings.Split(content, "\n")))
	if state.Error != "" {
		lines := []modalLine{
			{text: title, style: modalTitleStyle},
			{text: state.Error, style: modalBodyStyle},
		}
		if availableLines >= 3 {
			lines = append(lines, modalLine{text: "Esc close", style: modalHintStyle})
		}
		return renderModalOverlay(content, width, lines...)
	}
	if availableLines == 1 {
		return renderModalOverlay(content, width, modalLine{text: title, style: modalTitleStyle})
	}
	if len(state.Options) == 0 {
		lines := []modalLine{
			{text: title, style: modalTitleStyle},
			{text: "(no options)", style: modalBodyStyle},
		}
		if availableLines >= 3 {
			lines = append(lines, modalLine{text: "Esc cancel", style: modalHintStyle})
		}
		return renderModalOverlay(content, width,
			lines...,
		)
	}

	activeIdx := clamp(state.Index, 0, len(state.Options)-1)
	lines := []modalLine{{text: title, style: modalTitleStyle}}
	optionRows := maxInt(1, availableLines-2)
	includeHint := true
	if availableLines == 2 {
		optionRows = 1
		includeHint = false
	}
	start := rowWindowStart(len(state.Options), optionRows, activeIdx)
	end := minInt(len(state.Options), start+optionRows)
	for i := start; i < end; i++ {
		option := state.Options[i]
		lineStyle := modalOptionDefaultStyle
		label := "  " + option
		if i == activeIdx {
			lineStyle = modalOptionActiveStyle
			label = "> " + option
		}
		lines = append(lines, modalLine{text: label, style: lineStyle})
	}
	if includeHint {
		lines = append(lines, modalLine{text: "Up/Down select | Enter apply | Esc cancel", style: modalHintStyle})
	}
	return renderModalOverlay(content, width, lines...)
}

type modalLine struct {
	text  string
	style lipgloss.Style
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

const modalHorizontalPadding = 1

func renderModalOverlay(content string, width int, modalLines ...modalLine) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	innerWidth := 0
	for _, modalLine := range modalLines {
		innerWidth = maxInt(innerWidth, lipgloss.Width(modalLine.text))
	}

	mid := len(lines) / 2
	start := mid - len(modalLines)/2
	for i, modalLine := range modalLines {
		idx := start + i
		if idx < 0 || idx >= len(lines) {
			continue
		}

		if width <= 0 {
			lines[idx] = renderCenteredModalLine(modalLine.style, modalLine.text, innerWidth, width)
			continue
		}

		boxWidth := minInt(width, innerWidth+modalHorizontalPadding*2)
		xStart := maxInt(0, (width-boxWidth)/2)
		modalInner := maxInt(0, boxWidth-modalHorizontalPadding*2)
		modalSegment := renderModalLine(modalLine.style, modalLine.text, modalInner)
		overlayModalSegment(lines, idx, width, xStart, lipgloss.Width(modalSegment), modalSegment)
	}

	return strings.Join(lines, "\n")
}

func renderCenteredModalLine(style lipgloss.Style, text string, innerWidth, width int) string {
	content := renderModalLine(style, text, innerWidth)
	if width <= 0 {
		return content
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}

func renderModalLine(style lipgloss.Style, text string, innerWidth int) string {
	return style.Render(strings.Repeat(" ", modalHorizontalPadding) + pad(truncate(text, innerWidth), innerWidth) + strings.Repeat(" ", modalHorizontalPadding))
}

func overlayModalSegment(lines []string, idx, width, xStart, boxWidth int, segment string) {
	if idx < 0 || idx >= len(lines) {
		return
	}
	base := lines[idx]
	left := padStyledSegment(xansi.Cut(base, 0, xStart), xStart)
	rightWidth := maxInt(0, width-(xStart+boxWidth))
	right := padStyledSegment(xansi.Cut(base, xStart+boxWidth, width), rightWidth)
	lines[idx] = left + segment + right
}

func padStyledSegment(segment string, width int) string {
	currentWidth := lipgloss.Width(segment)
	if currentWidth >= width {
		return segment
	}
	return segment + strings.Repeat(" ", width-currentWidth)
}

func (m Model) toString() string {
	header := m.openViewHeader()
	footer := renderFileFooterBar(m.fileLineInfo(), m.footerText(), m.width, m.view.Kind == FileOutput)
	rowsHeight := m.viewRowsHeight()
	rows := fillLines(m.renderRows(rowsHeight), rowsHeight)
	parts := []string{header, m.commandBar()}
	parts = append(parts, strings.Join(rows, "\n"), footer)
	return strings.Join(parts, "\n")
}

func (m Model) openViewHeader() string {
	if m.view == nil {
		return renderFullWidthBar(headerStyle, "", m.width)
	}
	if m.view.Kind == FileHelp {
		return renderFullWidthBar(headerStyle, m.headerText(), m.width)
	}

	label := strings.TrimSpace(m.view.ResourceName)
	if label == "" {
		label = "RESOURCE"
	}
	items := []string{label}
	active := 0
	if m.view.hasMultiplePages() {
		for _, page := range m.view.Pages {
			items = append(items, page.Name)
		}
		active = 1 + clamp(m.view.ActivePage, 0, len(m.view.Pages)-1)
	}
	return renderHeaderMenuBar(items, active, m.width, true)
}

func (m Model) commandBar() string {
	if m.view == nil || m.view.Kind == FileHelp {
		return ""
	}
	return renderFullWidthBar(commandBarStyle, m.view.Title, m.width)
}

func renderFileFooterBar(left, right string, width int, withRightStat bool) string {
	if !withRightStat || strings.TrimSpace(right) == "" {
		return renderFullWidthBar(footerStyle, left, width)
	}
	if width <= 0 {
		return footerStyle.Render(left + " | " + right)
	}

	contentWidth := maxInt(0, width-2)
	sep := " | "
	sepWidth := lipgloss.Width(sep)
	rightWidth := lipgloss.Width(right)
	if rightWidth >= contentWidth {
		return renderFullWidthBar(footerStyle, right, width)
	}

	leftWidth := maxInt(0, contentWidth-rightWidth-sepWidth)
	leftText := truncate(left, leftWidth)
	content := leftText
	if leftWidth > 0 {
		content += sep
	}
	content += right

	plainWidth := lipgloss.Width(content)
	if plainWidth < contentWidth {
		content += strings.Repeat(" ", contentWidth-plainWidth)
	}
	return footerStyle.Render(content)
}

func (m Model) Header() string {
	context := strings.TrimSpace(m.context)
	namespace := strings.TrimSpace(m.namespace)
	if context == "" {
		context = "-"
	}
	if namespace == "" {
		namespace = "-"
	}
	return renderMainHeaderBar(context, namespace, m.width)
}

func renderMainHeaderBar(context, namespace string, width int) string {
	return renderHeaderMenuBar(
		[]string{context, namespace, "Enter view", "? help"},
		0,
		width,
		false,
	)
}

func renderHeaderMenuBar(items []string, active, width int, suppressFirstSeparator bool) string {
	normalStyle := headerStyle.Copy().Padding(0)
	activeStyle := activeHeaderTabStyle.Copy().Padding(0)
	plain, styled := renderMenuLine(items, active, func(i int, part string) string {
		if i == 0 || i == active {
			return activeStyle.Render(part)
		}
		return normalStyle.Render(part)
	}, normalStyle, suppressFirstSeparator)
	if width <= 0 {
		return styled
	}

	contentWidth := maxInt(0, width)
	if lipgloss.Width(plain) > contentWidth {
		return normalStyle.Render(pad(truncate(plain, contentWidth), contentWidth))
	}
	padding := contentWidth - lipgloss.Width(plain)
	if padding > 0 {
		styled += normalStyle.Render(strings.Repeat(" ", padding))
	}
	return styled
}

func renderMenuLine(items []string, active int, renderItem func(i int, part string) string, separatorStyle lipgloss.Style, suppressFirstSeparator bool) (string, string) {
	if len(items) == 0 {
		return "", ""
	}
	plainParts := make([]string, 0, len(items)*2-1)
	styledParts := make([]string, 0, len(items)*2-1)
	for i, item := range items {
		part := " " + item + " "
		plainParts = append(plainParts, part)
		styledParts = append(styledParts, renderItem(i, part))
		if i == len(items)-1 {
			continue
		}
		if suppressFirstSeparator && i == 0 {
			continue
		}
		if i == active-1 || i == active {
			continue
		}
		plainParts = append(plainParts, "|")
		styledParts = append(styledParts, separatorStyle.Render("|"))
	}
	return strings.Join(plainParts, ""), strings.Join(styledParts, "")
}
func renderFullWidthBar(style lipgloss.Style, text string, width int) string {
	if width <= 0 {
		return style.Render(text)
	}
	contentWidth := maxInt(0, width-2)
	content := pad(truncate(text, contentWidth), contentWidth)
	return style.Render(content)
}

func renderFooterBar(text, status string, width int) string {
	if status == "" {
		return renderFullWidthBar(footerStyle, text, width)
	}
	if width <= 0 {
		return footerStyle.Render(text + " | " + status)
	}

	sep := " | "
	contentWidth := maxInt(0, width-2)
	plainRight := sep + status
	rightWidth := lipgloss.Width(plainRight)

	if rightWidth >= contentWidth {
		return renderFullWidthBar(footerStyle, plainRight, width)
	}

	leftWidth := maxInt(0, contentWidth-rightWidth)
	left := pad(truncate(text, leftWidth), leftWidth)
	content := left + sep + refreshStatusStyle.Render(status)
	return footerStyle.Copy().Padding(0, 1).Render(content)
}

func (m Model) Footer() string {
	r := m.currentRow()
	status := m.refreshStatusText()
	if r == nil {
		return renderFooterBar("No items", status, m.width)
	}
	action := m.effectiveAction(r)
	args := m.rowActionArgs(r, action)
	footerText := "kubectl " + strings.Join(args, " ")
	row := renderFooterBar(footerText, status, m.width)
	if m.footerHeight == 1 {
		return row
	}

	rows := []string{row}
	for i := 1; i < m.footerHeight; i++ {
		rows = append(rows, renderFullWidthBar(footerStyle, "", m.width))
	}
	return strings.Join(rows, "\n")
}

func (m Model) ViewRows(height int) string {
	rows := m.rowsByPane[m.activePane]
	if len(rows) == 0 {
		return strings.Repeat("\n", maxInt(0, height-1)) + "No items"
	}

	cursor := clamp(m.cursorByPane[m.activePane], 0, len(rows)-1)
	start := rowWindowStart(len(rows), height, cursor)
	end := minInt(len(rows), start+height)

	newRows := make([]string, 0, height)
	for i := start; i < end; i++ {
		newRows = append(newRows, m.renderRow(rows[i], i == cursor))
	}
	newRows = fillLines(newRows, height)
	return strings.Join(newRows, "\n")
}

func fillLines(lines []string, height int) []string {
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

func rowWindowStart(rowsLen, height, cursor int) int {
	if rowsLen <= 0 || height <= 0 {
		return 0
	}
	maxStart := maxInt(0, rowsLen-height)
	start := maxInt(0, cursor-height/2)
	end := minInt(rowsLen, start+height)
	if end-start < height {
		start = maxInt(0, end-height)
	}
	return clamp(start, 0, maxStart)
}

func (m Model) headerText() string {
	if m.view.Kind == FileHelp {
		return "HELP | Keybindings"
	}
	resourceLabel := "RESOURCE"
	if name := strings.TrimSpace(m.view.ResourceName); name != "" {
		resourceLabel = name
	}
	if m.view.hasMultiplePages() {
		return fmt.Sprintf("%s | %s", resourceLabel, m.view.pageTabs())
	}
	return resourceLabel
}

func (m Model) fileLineInfo() string {
	if m.view == nil || m.view.Kind != FileOutput {
		return ""
	}
	return fmt.Sprintf("line %d/%d col %d", minInt(len(m.view.Rows), m.view.Scroll+1), len(m.view.Rows), m.view.ColScroll+1)
}

func (m Model) footerText() string {
	if m.view.Kind == FileHelp {
		return "Tab/Shift+Tab context | arrows rows | Space select | Enter resource view"
	}
	if m.view.Kind != FileHelp && m.view.hasMultiplePages() {
		return "Tab/Shift+Tab cycle context (applicable views)"
	}
	return ""
}

func (v *View) pageTabs() string {
	if v == nil || len(v.Pages) == 0 {
		return ""
	}
	names := make([]string, 0, len(v.Pages))
	for _, page := range v.Pages {
		names = append(names, page.Name)
	}
	return strings.Join(names, " | ")
}

func (m Model) renderRows(rowsHeight int) []string {
	start := clamp(m.view.Scroll, 0, m.maxViewScroll())
	end := minInt(len(m.view.Rows), start+rowsHeight)
	rowsRows := make([]string, 0, rowsHeight)
	lineNumberWidth := len(fmt.Sprintf("%d", maxInt(1, len(m.view.Rows))))
	contentWidth := maxInt(1, m.width-lineNumberWidth-4)
	pageName := ""
	if m.view != nil && m.view.Kind == FileOutput {
		pageName = m.view.pageName()
	}

	for i := start; i < end; i++ {
		line := visibleSegment(m.view.Rows[i], m.view.ColScroll, contentWidth)
		line = highlightResourceLine(pageName, line)
		prefix := rowPrefix(i, lineNumberWidth)
		rowsRows = append(rowsRows, prefix+line)
	}
	return rowsRows
}

func (m Model) renderRow(r Row, focused bool) string {
	if r.Separator {
		return ""
	}
	pane := m.activePane
	return m.renderRowForPane(r, pane, focused)
}

func (m Model) renderRowForPane(r Row, pane int, focused bool) string {
	if r.Separator {
		return ""
	}
	state := rowState{
		Focused:     focused,
		Selected:    m.selected[pane][r.Key],
		Describable: m.canDescribeRow(&r),
	}
	full := m.rowScrollableContentForPane(r, pane, focused)
	content := visibleSegment(full, m.mainColScroll[pane], maxInt(1, m.width))
	if !state.Focused && !state.Selected {
		content = colorizeMainVisibleSegment(content, metadataOpenAtOffset(full, m.mainColScroll[pane]))
	}
	return m.styleRowContent(content, state)
}

var metadataChunkPattern = regexp.MustCompile(`\{[^{}]*(\}|$)`)

func colorizeMainVisibleSegment(content string, startsInsideMetadata bool) string {
	if content == "" {
		return ""
	}

	hasContinuation := strings.HasSuffix(content, "~")
	base := content
	if hasContinuation {
		base = strings.TrimSuffix(base, "~")
	}

	styled := base
	if startsInsideMetadata {
		if idx := strings.Index(styled, "}"); idx >= 0 {
			styled = rowMetadataStyle.Render(styled[:idx+1]) + styled[idx+1:]
		} else {
			styled = rowMetadataStyle.Render(styled)
		}
	}

	styled = metadataChunkPattern.ReplaceAllStringFunc(styled, func(chunk string) string {
		return rowMetadataStyle.Render(chunk)
	})
	if hasContinuation {
		styled += rowContinuationStyle.Render("~")
	}
	return styled
}

func metadataOpenAtOffset(content string, offset int) bool {
	if content == "" {
		return false
	}
	r := []rune(content)
	limit := clamp(offset, 0, len(r))
	depth := 0
	for i := 0; i < limit; i++ {
		switch r[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth > 0
}

func (m Model) rowScrollableContentForPane(r Row, pane int, focused bool) string {
	state := rowState{
		Focused:     focused,
		Selected:    m.selected[pane][r.Key],
		Describable: m.canDescribeRow(&r),
	}
	content := plainRowContent(r)
	if state.Selected {
		return content
	}
	return withSelectionMarkerOnRowState(content, rowSelectionMarker(state), true)
}

func plainRowContent(r Row) string {
	if strings.TrimSpace(r.Plain) != "" {
		return r.Plain
	}
	if strings.TrimSpace(r.PlainText) != "" {
		return r.PlainText
	}
	if strings.TrimSpace(r.Text) != "" {
		return r.Text
	}
	return r.Name
}

func rowSelectionMarker(state rowState) string {
	if !state.Describable {
		return disabledMarker
	}
	return "[ ]"
}

func rowContent(r Row, state rowState) string {
	rowContent := r.Text
	if state.Focused || state.Selected {
		rowContent = r.Plain
		if rowContent == "" {
			rowContent = r.PlainText
		}
	}
	if strings.TrimSpace(rowContent) == "" {
		return r.Name
	}
	return rowContent
}

func (m Model) styleRowContent(content string, state rowState) string {
	if !state.Focused && !state.Selected {
		return content
	}

	width := maxInt(1, m.width)
	content = pad(truncate(content, width), width)
	if state.Focused {
		if !state.Describable {
			return disabledFocusedRowStyle.Render(content)
		}
		return focusedCellStyle.Render(content)
	}
	if !state.Describable {
		return disabledSelectedRowStyle.Render(content)
	}
	return selectedRowStyle.Render(content)
}

func withSelectionMarkerOnRow(rowContent, marker string) string {
	return withSelectionMarkerOnRowState(rowContent, marker, marker == "[ ]")
}

func withSelectionMarkerOnRowState(rowContent, marker string, hideUncheckedMarker bool) string {
	for _, branch := range []string{"├─ ", "└─ "} {
		if idx := strings.Index(rowContent, branch); idx >= 0 {
			insertPos := idx + len(branch)
			tail := rowContent[insertPos:]
			if hideUncheckedMarker {
				if emoji, rest, ok := consumeLeadingEmoji(tail); ok {
					return rowContent[:insertPos] + emoji + " " + rest
				}
			}
			return rowContent[:insertPos] + marker + " " + tail
		}
	}
	if hideUncheckedMarker {
		if emoji, rest, ok := consumeLeadingEmoji(rowContent); ok {
			return emoji + " " + rest
		}
	}
	return marker + " " + rowContent
}

func consumeLeadingEmoji(s string) (string, string, bool) {
	trimmed := strings.TrimLeft(s, " ")
	if trimmed == "" {
		return "", s, false
	}
	r, size := utf8.DecodeRuneInString(trimmed)
	if r == utf8.RuneError || r <= unicode.MaxASCII || unicode.IsLetter(r) || unicode.IsNumber(r) {
		return "", s, false
	}
	rest := strings.TrimLeft(trimmed[size:], " ")
	return string(r), rest, true
}
