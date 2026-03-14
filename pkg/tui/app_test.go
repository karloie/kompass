package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	kube "github.com/karloie/kompass/pkg/kube"
)

func TestTabAndShiftTabSwitchPanes(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []row{{Key: "selector/a"}}
	m.rowsByPane[1] = []row{{Key: "single/a"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m1 := updated.(model)
	if m1.activePane != 1 {
		t.Fatalf("expected active pane 1 after Tab, got %d", m1.activePane)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m2 := updated.(model)
	if m2.activePane != 0 {
		t.Fatalf("expected active pane 0 after Shift+Tab, got %d", m2.activePane)
	}
}

func TestTabSkipsSingleWhenEmpty(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.activePane = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m1 := updated.(model)
	if m1.activePane != 0 {
		t.Fatalf("expected Tab to stay on pane 0 when no single rows, got %d", m1.activePane)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m2 := updated.(model)
	if m2.activePane != 0 {
		t.Fatalf("expected Shift+Tab to stay on pane 0 when no single rows, got %d", m2.activePane)
	}
}

func TestTabSkipsSelectorWhenEmpty(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = nil
	m.rowsByPane[1] = []row{{Key: "single/a"}}
	m.activePane = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m1 := updated.(model)
	if m1.activePane != 1 {
		t.Fatalf("expected Tab to move to pane 1 when selector is empty, got %d", m1.activePane)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m2 := updated.(model)
	if m2.activePane != 1 {
		t.Fatalf("expected Shift+Tab to stay on pane 1 when selector is empty, got %d", m2.activePane)
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m3 := updated.(model)
	if m3.activePane != 1 {
		t.Fatalf("expected key 1 to fall back to pane 1 when selector is empty, got %d", m3.activePane)
	}
}

func TestQuestionMarkOpensHelpFile(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m1 := updated.(model)
	if m1.file == nil {
		t.Fatalf("expected help file to open")
	}
	if m1.file.Kind != fileHelp {
		t.Fatalf("expected help file kind, got %q", m1.file.Kind)
	}
}

func TestOpenYAMLFilePrefersResourceData(t *testing.T) {
	r := row{Key: "pod/ns/foo", Type: "pod", Name: "foo", Status: "Running", Metadata: map[string]any{"name": "foo"}}
	res := map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": "foo"}}
	file := openYAMLFile(r, &kube.Resource{Resource: res})
	if file == nil {
		t.Fatalf("expected yaml file")
	}
	joined := strings.Join(file.Lines, "\n")
	if !strings.Contains(joined, "kind: Pod") {
		t.Fatalf("expected yaml file to include resource kind, got:\n%s", joined)
	}
}

func TestEscClosesFileBeforeQuit(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.file = openHelpFile()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m1 := updated.(model)
	if m1.file != nil {
		t.Fatalf("expected Esc to close file first")
	}
	if cmd != nil {
		t.Fatalf("expected no quit command while closing file")
	}

	_, cmd = m1.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatalf("expected second Esc to quit app")
	}
}

func TestDoubleEnterReturnsToSelector(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []row{{Key: "pod/ns/a", Type: "pod", Name: "a", Status: "Running"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := updated.(model)
	if cmd != nil {
		t.Fatalf("expected Enter to open file without quitting")
	}
	if m1.file == nil {
		t.Fatalf("expected first Enter to open file")
	}

	updated, cmd = m1.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(model)
	if cmd != nil {
		t.Fatalf("expected second Enter to close file without quitting")
	}
	if m2.file != nil {
		t.Fatalf("expected second Enter to return to selector (close file)")
	}
}

func TestFooterSummaryByColumn(t *testing.T) {
	r := &row{
		Key:    "pod/ns/foo",
		Type:   "pod",
		Name:   "foo",
		Status: "Running",
		Depth:  2,
		Metadata: map[string]any{
			"reason":    "CrashLoopBackOff",
			"namespace": "ns",
		},
	}

	if got := footerSummary("ctx", "ns", r); !strings.Contains(got, "key=pod/ns/foo") {
		t.Fatalf("expected footer summary to include key, got %q", got)
	}
}

func TestFileSearchFindsMatchesAndJumps(t *testing.T) {
	r := row{Key: "k", Type: "pod", Name: "foo"}
	resource := &kube.Resource{Resource: map[string]any{
		"kind": "Pod",
		"metadata": map[string]any{
			"name": "foo",
		},
	}}
	m := newRun(Options{Mode: ModeSelector})
	m.file = openYAMLFile(r, resource)
	m.file.SearchMode = true
	m.file.SearchQuery = "name"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := updated.(model)
	if m1.file.SearchMode {
		t.Fatalf("expected search mode to close after Enter")
	}
	if len(m1.file.MatchLines) == 0 {
		t.Fatalf("expected at least one search match")
	}

	before := m1.file.ActiveMatch
	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m2 := updated.(model)
	if len(m2.file.MatchLines) > 1 && m2.file.ActiveMatch == before {
		t.Fatalf("expected n to change active match when multiple matches exist")
	}
}

func TestEscInSearchModeClosesSearchNotFile(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.file = openHelpFile()
	m.file.SearchMode = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m1 := updated.(model)
	if m1.file == nil {
		t.Fatalf("expected file to remain open when Esc closes search mode")
	}
	if m1.file.SearchMode {
		t.Fatalf("expected search mode to be closed")
	}
}

func TestCtrlASelectsAllRowsInActivePane(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []row{{Key: "a"}, {Key: "b"}, {Key: "c"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m1 := updated.(model)
	if len(m1.selected[0]) != 3 {
		t.Fatalf("expected 3 selected rows after Ctrl+A, got %d", len(m1.selected[0]))
	}
}

func TestDoubleDownJumpsFiveRows(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = make([]row, 30)
	t0 := time.Unix(0, 0)
	step := 0
	m.now = func() time.Time {
		if step == 0 {
			step++
			return t0
		}
		return t0.Add(150 * time.Millisecond)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m1 := updated.(model)
	if m1.cursorByPane[0] != 1 {
		t.Fatalf("expected first down to move 1 row, got %d", m1.cursorByPane[0])
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := updated.(model)
	if m2.cursorByPane[0] != 6 {
		t.Fatalf("expected second down to jump to row 6, got %d", m2.cursorByPane[0])
	}
}

func TestDoubleUpJumpsFiveRows(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = make([]row, 30)
	m.cursorByPane[0] = 20
	t0 := time.Unix(0, 0)
	step := 0
	m.now = func() time.Time {
		if step == 0 {
			step++
			return t0
		}
		return t0.Add(150 * time.Millisecond)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m1 := updated.(model)
	if m1.cursorByPane[0] != 19 {
		t.Fatalf("expected first up to move 1 row, got %d", m1.cursorByPane[0])
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyUp})
	m2 := updated.(model)
	if m2.cursorByPane[0] != 14 {
		t.Fatalf("expected second up to jump to row 14, got %d", m2.cursorByPane[0])
	}
}

func TestNavJumpResetsAfterOtherKey(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = make([]row, 30)
	t0 := time.Unix(0, 0)
	step := 0
	m.now = func() time.Time {
		if step == 0 {
			step++
			return t0
		}
		return t0.Add(150 * time.Millisecond)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m1 := updated.(model)

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m2 := updated.(model)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyDown})
	m3 := updated.(model)
	if m3.cursorByPane[0] != 2 {
		t.Fatalf("expected down after other key to move 1 row, got %d", m3.cursorByPane[0])
	}
}

func TestHeldDownDoesNotTriggerJump(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = make([]row, 30)
	t0 := time.Unix(0, 0)
	step := 0
	m.now = func() time.Time {
		if step == 0 {
			step++
			return t0
		}
		return t0.Add(40 * time.Millisecond)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m1 := updated.(model)
	if m1.cursorByPane[0] != 1 {
		t.Fatalf("expected first down to move 1 row, got %d", m1.cursorByPane[0])
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := updated.(model)
	if m2.cursorByPane[0] != 2 {
		t.Fatalf("expected held down repeat to move 1 row, got %d", m2.cursorByPane[0])
	}
}

func TestRowWindowStartAnchorsOnJumpDirection(t *testing.T) {
	if got := rowWindowStart(30, 8, 11, 1); got != 11 {
		t.Fatalf("expected down jump anchor to place cursor row at top start=11, got %d", got)
	}
	if got := rowWindowStart(30, 8, 11, -1); got != 4 {
		t.Fatalf("expected up jump anchor to place cursor row at bottom start=4, got %d", got)
	}
}

func TestNavJumpSetsAnchorDirection(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = make([]row, 30)
	t0 := time.Unix(0, 0)
	step := 0
	m.now = func() time.Time {
		if step == 0 {
			step++
			return t0
		}
		return t0.Add(150 * time.Millisecond)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m1 := updated.(model)
	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := updated.(model)
	if m2.navAnchorDir != 1 {
		t.Fatalf("expected down jump to set navAnchorDir=1, got %d", m2.navAnchorDir)
	}

	t1 := time.Unix(0, 0)
	step2 := 0
	m2.now = func() time.Time {
		if step2 == 0 {
			step2++
			return t1
		}
		return t1.Add(150 * time.Millisecond)
	}
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyUp})
	m3 := updated.(model)
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyUp})
	m4 := updated.(model)
	if m4.navAnchorDir != -1 {
		t.Fatalf("expected up jump to set navAnchorDir=-1, got %d", m4.navAnchorDir)
	}
}

func TestOQuitsAndEnablesOutput(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []row{{Key: "pod/ns/api"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m1 := updated.(model)
	if cmd == nil {
		t.Fatalf("expected o to quit")
	}
	if !m1.emitSelection {
		t.Fatalf("expected o to enable selection output")
	}
	keys := m1.keysForOutput()
	if len(keys) != 1 || keys[0] != "pod/ns/api" {
		t.Fatalf("expected output keys to include current row key, got %#v", keys)
	}
}

func TestRenderRowFileedLongNameStaysSingleLine(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 24
	r := row{Key: "k", Name: "deployment/applikasjonsplattform/ad-explore-webservice-config"}

	row := m.renderRow(r, true)
	if strings.Contains(row, "\n") {
		t.Fatalf("expected fileed row to render as a single row")
	}
	if got := lipgloss.Width(row); got != m.width {
		t.Fatalf("expected fileed row to invert full width %d, got %d", m.width, got)
	}
}

func TestRenderRowSelectedUsesPlainText(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 40
	r := row{Key: "k", Text: "└─ colored-row", PlainText: "└─ plain-row"}
	m.selected[0][r.Key] = true

	row := m.renderRow(r, false)
	if strings.Contains(row, "colored-row") {
		t.Fatalf("expected selected row to use PlainText, got %q", row)
	}
	if !strings.Contains(row, "plain-row") {
		t.Fatalf("expected selected row to include PlainText content, got %q", row)
	}
}

func TestRenderRowSelectedFillsFullWidth(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 32
	r := row{Key: "k", PlainText: "└─ pod short"}
	m.selected[0][r.Key] = true

	row := m.renderRow(r, false)
	if got := lipgloss.Width(row); got != m.width {
		t.Fatalf("expected selected row width %d, got %d", m.width, got)
	}
}

func TestViewHeaderFillsViewportWidth(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 64

	header := m.viewHeader()
	if got := lipgloss.Width(header); got != m.width {
		t.Fatalf("expected header width %d, got %d", m.width, got)
	}
}

func TestViewFooterFillsViewportWidth(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 64
	m.rowsByPane[0] = []row{{Key: "pod/ns/a", Type: "pod", Name: "a", Status: "Running"}}

	footer := m.viewFooter()
	if got := lipgloss.Width(footer); got != m.width {
		t.Fatalf("expected footer width %d, got %d", m.width, got)
	}
}

func TestWithSelectionMarkerOnTreeBranch(t *testing.T) {
	row := withSelectionMarkerOnRow("└─ service child", "[ ]")
	if !strings.Contains(row, "└─ [ ] service child") {
		t.Fatalf("expected marker inserted after branch prefix, got %q", row)
	}

	root := withSelectionMarkerOnRow("deployment root", "[x]")
	if !strings.HasPrefix(root, "[x] deployment root") {
		t.Fatalf("expected marker prefixed for root row, got %q", root)
	}
}

func TestFlattenTreesUsesASCIITreeLines(t *testing.T) {
	trees := &kube.Trees{
		Nodes: map[string]*kube.Resource{},
		Trees: []*kube.Tree{
			{
				Key:  "deploy/ns/root",
				Type: "deployment",
				Meta: map[string]any{"name": "root"},
				Children: []*kube.Tree{
					{Key: "svc/ns/child", Type: "service", Meta: map[string]any{"name": "child"}},
				},
			},
		},
	}

	rows := flattenTrees(trees)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if !strings.Contains(rows[0].Text, "root") {
		t.Fatalf("expected root row to include rendered root name, got %q", rows[0].Text)
	}
	if !strings.Contains(rows[1].Text, "child") {
		t.Fatalf("expected child row to include rendered child name, got %q", rows[1].Text)
	}
	if !strings.Contains(rows[1].Text, "└") && !strings.Contains(rows[1].Text, "├") {
		t.Fatalf("expected child row to include ASCII branch prefix, got %q", rows[1].Text)
	}
}

func TestEscClearsSelectionBeforeQuit(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []row{{Key: "a"}}
	m.selected[0]["a"] = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m1 := updated.(model)
	if cmd != nil {
		t.Fatalf("expected no quit command when Esc clears selection")
	}
	if len(m1.selected[0]) != 0 {
		t.Fatalf("expected selection cleared, got %d selected", len(m1.selected[0]))
	}

	_, cmd = m1.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatalf("expected second Esc to quit")
	}
}

func TestViewerFileActiveMatchLine(t *testing.T) {
	file := &viewerFile{MatchLines: []int{3, 7, 11}, ActiveMatch: 1}
	if got := file.activeMatchLine(); got != 7 {
		t.Fatalf("expected active match row 7, got %d", got)
	}

	file.ActiveMatch = 99
	if got := file.activeMatchLine(); got != -1 {
		t.Fatalf("expected out-of-range active match to return -1, got %d", got)
	}
}

func TestContainsInt(t *testing.T) {
	values := []int{2, 4, 6}
	if !containsInt(values, 4) {
		t.Fatalf("expected containsInt to find value")
	}
	if containsInt(values, 5) {
		t.Fatalf("expected containsInt to not find missing value")
	}
}

func TestHighlightSearchTerm(t *testing.T) {
	row := "metadata.name: petshop-db"

	if got := highlightSearchTerm(row, "", false); got != row {
		t.Fatalf("expected empty query to keep row unchanged")
	}

	inactive := highlightSearchTerm(row, "name", false)
	active := highlightSearchTerm(row, "name", true)

	if !strings.Contains(inactive, "name") {
		t.Fatalf("expected inactive highlighting to preserve searched term")
	}
	if !strings.Contains(active, "name") {
		t.Fatalf("expected active highlighting to preserve searched term")
	}
	if got := highlightSearchTerm(row, "missing", true); got != row {
		t.Fatalf("expected missing query to keep row unchanged")
	}
}

func TestFileHomeEndNavigation(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 20
	m.file = &viewerFile{Kind: fileYAML, Lines: []string{"abcdefghijklmnopqrstuvwxyz", "b", "c", "d"}, ColScroll: 6, Scroll: 2}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m1 := updated.(model)
	if m1.file.ColScroll != 0 {
		t.Fatalf("expected Home to jump to row start, got %d", m1.file.ColScroll)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m2 := updated.(model)
	if m2.file.ColScroll == 0 {
		t.Fatalf("expected End to jump to row end, got %d", m2.file.ColScroll)
	}
}

func TestFileGAndGNavigation(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.file = &viewerFile{Kind: fileYAML, Lines: []string{"a", "b", "c", "d"}, Scroll: 2}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m1 := updated.(model)
	if m1.file.Scroll != 0 {
		t.Fatalf("expected g to jump to top, got %d", m1.file.Scroll)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m2 := updated.(model)
	if m2.file.Scroll != 3 {
		t.Fatalf("expected G to jump to bottom, got %d", m2.file.Scroll)
	}
}

func TestFileGutterMarker(t *testing.T) {
	matchLines := []int{2, 5, 8}

	if got := fileGutterMarker(5, matchLines, 8); got != "*" {
		t.Fatalf("expected match marker '*', got %q", got)
	}
	if got := fileGutterMarker(8, matchLines, 8); got != ">" {
		t.Fatalf("expected active marker '>', got %q", got)
	}
	if got := fileGutterMarker(3, matchLines, 8); got != " " {
		t.Fatalf("expected no marker for non-match row, got %q", got)
	}
}

func TestResolveEditorCommandDefaultsToReadOnlyVi(t *testing.T) {
	bin, args := resolveEditorCommand("", "/tmp/sample.yaml")
	if bin != "vi" {
		t.Fatalf("expected default editor vi, got %q", bin)
	}
	if !containsString(args, "-R") {
		t.Fatalf("expected vi to include -R for read-only mode, args=%v", args)
	}
	if args[len(args)-1] != "/tmp/sample.yaml" {
		t.Fatalf("expected file path as last arg, args=%v", args)
	}
}

func TestResolveEditorCommandForNanoUsesViewerMode(t *testing.T) {
	bin, args := resolveEditorCommand("nano", "/tmp/sample.yaml")
	if bin != "nano" {
		t.Fatalf("expected nano editor, got %q", bin)
	}
	if !containsString(args, "-v") {
		t.Fatalf("expected nano to include -v for view mode, args=%v", args)
	}
}

func TestResolveEditorCommandForUnknownKeepsArgs(t *testing.T) {
	bin, args := resolveEditorCommand("cat -n", "/tmp/sample.yaml")
	if bin != "cat" {
		t.Fatalf("expected cat editor command, got %q", bin)
	}
	if containsString(args, "-R") || containsString(args, "-v") {
		t.Fatalf("expected unknown editor args to remain unchanged, args=%v", args)
	}
	if !containsString(args, "-n") {
		t.Fatalf("expected existing editor args to be preserved, args=%v", args)
	}
}

func TestFileHorizontalPanning(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.file = &viewerFile{Kind: fileYAML, Lines: []string{"abcdefghijklmnopqrstuvwxyz"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m1 := updated.(model)
	if m1.file.ColScroll != 4 {
		t.Fatalf("expected right to increase col scroll to 4, got %d", m1.file.ColScroll)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m2 := updated.(model)
	if m2.file.ColScroll != 0 {
		t.Fatalf("expected left to decrease col scroll to 0, got %d", m2.file.ColScroll)
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m3 := updated.(model)
	if m3.file.ColScroll != 0 {
		t.Fatalf("expected left at boundary to stay at 0, got %d", m3.file.ColScroll)
	}
}

func TestVisibleSegment(t *testing.T) {
	if got := visibleSegment("abcdef", 0, 3); got != "ab~" {
		t.Fatalf("expected truncated segment with continuation marker, got %q", got)
	}
	if got := visibleSegment("abcdef", 2, 3); got != "cd~" {
		t.Fatalf("expected scrolled segment with continuation marker, got %q", got)
	}
	if got := visibleSegment("abcdef", 10, 3); got != "" {
		t.Fatalf("expected empty segment when scrolled past content, got %q", got)
	}
}

func TestApplyFileSearchKeepsActiveMatchVisible(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 18
	m.file = &viewerFile{
		Kind:        fileYAML,
		Lines:       []string{"abcdefghijTARGETxyz"},
		SearchQuery: "target",
		ColScroll:   0,
	}

	m.applyFileSearch()
	if len(m.file.MatchLines) != 1 {
		t.Fatalf("expected one match row, got %d", len(m.file.MatchLines))
	}
	if m.file.ColScroll == 0 {
		t.Fatalf("expected horizontal auto-pan to reveal active match")
	}
}

func TestJumpFileMatchKeepsNextMatchVisible(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 20
	m.file = &viewerFile{
		Kind:        fileYAML,
		Lines:       []string{"TARGET", "abcdefghijklmnopTARGET"},
		SearchQuery: "target",
		MatchLines:  []int{0, 1},
		ActiveMatch: 0,
		ColScroll:   0,
	}

	m.jumpFileMatch(1)
	if m.file.ActiveMatch != 1 {
		t.Fatalf("expected active match to move to second row, got %d", m.file.ActiveMatch)
	}
	if m.file.ColScroll == 0 {
		t.Fatalf("expected horizontal auto-pan for second match")
	}
}

func TestMatchColumn(t *testing.T) {
	if got := matchColumn("abcDefG", "def"); got != 3 {
		t.Fatalf("expected match at column 3, got %d", got)
	}
	if got := matchColumn("abcdef", "zzz"); got != -1 {
		t.Fatalf("expected no match to return -1, got %d", got)
	}
}

func TestSlashPreservesPreviousSearchQuery(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.file = &viewerFile{Kind: fileYAML, Lines: []string{"a"}, SearchQuery: "name"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m1 := updated.(model)
	if !m1.file.SearchMode {
		t.Fatalf("expected slash to enter search mode")
	}
	if m1.file.SearchQuery != "name" {
		t.Fatalf("expected slash to preserve prior query, got %q", m1.file.SearchQuery)
	}
}

func TestCtrlUClearsSearchQuery(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.file = &viewerFile{Kind: fileYAML, Lines: []string{"a"}, SearchMode: true, SearchQuery: "status"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m1 := updated.(model)
	if m1.file.SearchQuery != "" {
		t.Fatalf("expected Ctrl+U to clear search query, got %q", m1.file.SearchQuery)
	}
}

func TestFormatSelectionOutputPlain(t *testing.T) {
	out, err := formatSelectionOutput([]string{"pod/ns/a", "svc/ns/b"}, false)
	if err != nil {
		t.Fatalf("expected plain formatting without error, got %v", err)
	}
	if out != "pod/ns/a\nsvc/ns/b\n" {
		t.Fatalf("unexpected plain output: %q", out)
	}
}

func TestFormatSelectionOutputJSON(t *testing.T) {
	out, err := formatSelectionOutput([]string{"pod/ns/a", "svc/ns/b"}, true)
	if err != nil {
		t.Fatalf("expected JSON formatting without error, got %v", err)
	}
	if out != "[\"pod/ns/a\",\"svc/ns/b\"]\n" {
		t.Fatalf("unexpected JSON output: %q", out)
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
