package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	kube "github.com/karloie/kompass/pkg/kube"
)

func TestTabAndShiftTabJumpBetweenRoots(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{
		{Key: "root/a", Depth: 0},
		{Key: "child/a1", Depth: 1},
		{Key: "root/b", Depth: 0},
		{Key: "child/b1", Depth: 1},
	}
	m.cursorByPane[0] = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m1 := updated.(Model)
	if m1.cursorByPane[0] != 2 {
		t.Fatalf("expected Tab to jump to next root row (2), got %d", m1.cursorByPane[0])
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m2 := updated.(Model)
	if m2.cursorByPane[0] != 0 {
		t.Fatalf("expected Shift+Tab to jump to previous root row (0), got %d", m2.cursorByPane[0])
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m3 := updated.(Model)
	if m3.cursorByPane[0] != 2 {
		t.Fatalf("expected Shift+Tab to wrap to last root row (2), got %d", m3.cursorByPane[0])
	}
}

func TestTabWrapsWhenNoFurtherRoot(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "root/a", Depth: 0}, {Key: "child/a1", Depth: 1}}
	m.cursorByPane[0] = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m1 := updated.(Model)
	if m1.cursorByPane[0] != 0 {
		t.Fatalf("expected Tab to wrap to first root row (0), got %d", m1.cursorByPane[0])
	}
}

func TestOneAndTwoSwitchPanes(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "selector/a"}}
	m.rowsByPane[1] = []Row{{Key: "single/a"}}
	m.activePane = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m1 := updated.(Model)
	if m1.activePane != 1 {
		t.Fatalf("expected key 2 to switch to pane 1, got %d", m1.activePane)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m2 := updated.(Model)
	if m2.activePane != 0 {
		t.Fatalf("expected key 1 to switch to pane 0, got %d", m2.activePane)
	}
}

func TestQuestionMarkOpensHelpFile(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m1 := updated.(Model)
	if m1.view == nil {
		t.Fatalf("expected help view to open")
	}
	if m1.view.Kind != FileHelp {
		t.Fatalf("expected help view kind, got %q", m1.view.Kind)
	}
}

func TestViewDescribeIncludesCommand(t *testing.T) {
	r := Row{Key: "pod/ns/foo", Type: "pod", Name: "foo", Status: "Running", Metadata: map[string]any{"name": "foo"}}
	view := viewDescribe(r, "ctx-a", "ns")
	if view == nil {
		t.Fatalf("expected describe view")
	}
	if view.Title != "kubectl --context ctx-a describe pod foo -n ns" {
		t.Fatalf("expected command in title, got %q", view.Title)
	}
	joined := strings.Join(view.Rows, "\n")
	if strings.Contains(joined, "$ kubectl --context ctx-a describe pod foo -n ns") {
		t.Fatalf("expected command removed from output body, got:\n%s", joined)
	}
}

func TestEscClosesFileBeforeQuit(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.view = viewHelp()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m1 := updated.(Model)
	if m1.view != nil {
		t.Fatalf("expected Esc to close view first")
	}
	if cmd != nil {
		t.Fatalf("expected no quit command while closing view")
	}

	_, cmd = m1.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatalf("expected second Esc to quit app")
	}
}

func TestDoubleEnterReturnsToSelector(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "pod/ns/a", Type: "pod", Name: "a", Status: "Running"}}
	m.resources["pod/ns/a"] = &kube.Resource{Key: "pod/ns/a", Type: "pod"}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := updated.(Model)
	if cmd != nil {
		t.Fatalf("expected Enter to open view without quitting")
	}
	if m1.view == nil {
		t.Fatalf("expected first Enter to open view")
	}

	updated, cmd = m1.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	if cmd != nil {
		t.Fatalf("expected second Enter to close view without quitting")
	}
	if m2.view != nil {
		t.Fatalf("expected second Enter to return to selector (close view)")
	}
}

func TestEnterDoesNotDescribeUnsupportedRow(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "pod/ns/a/container/0/environment/env/0", Type: "env", Name: "API_KEY"}}
	m.resources["pod/ns/a"] = &kube.Resource{Key: "pod/ns/a", Type: "pod"}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := updated.(Model)
	if cmd != nil {
		t.Fatalf("expected Enter on unsupported row to not quit")
	}
	if m1.view != nil {
		t.Fatalf("expected Enter on unsupported row to keep selector view")
	}
}

func TestFooterSummaryByColumn(t *testing.T) {
	r := &Row{
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

	row := withSelectionMarkerOnRow("└─ service child", "[ ]")
	if !strings.Contains(row, "└─ [ ] service child") {
		t.Fatalf("expected marker inserted after branch prefix, got %q", row)
	}
	if got := footerSummary("ctx", "ns", r); !strings.Contains(got, "key=pod/ns/foo") {
		t.Fatalf("expected footer summary to include key, got %q", got)
	}
}

func TestWithSelectionMarkerOnRowEmbedsEmojiWhenUnchecked(t *testing.T) {
	line := withSelectionMarkerOnRow("└─ 💬 secret value", "[ ]")
	if !strings.Contains(line, "└─ 💬 secret value") {
		t.Fatalf("expected unchecked marker to keep emoji without brackets, got %q", line)
	}
}

func TestWithSelectionMarkerOnRowKeepsEmojiOutsideWhenChecked(t *testing.T) {
	line := withSelectionMarkerOnRow("└─ 💬 secret value", "[x]")
	if !strings.Contains(line, "└─ [x] 💬 secret value") {
		t.Fatalf("expected checked marker to keep emoji outside marker, got %q", line)
	}
}

func TestRenderRowShowsExplicitMarkerForNonK8sNode(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 120
	r := Row{Key: "pod/ns/a/container/0/environment/env/0", Text: "└─ 💬 env API_KEY", Plain: "└─ 💬 env API_KEY"}

	line := m.renderRow(r, false)
	if strings.Contains(line, "[-]") {
		t.Fatalf("expected non-k8s row to hide selector marker, got %q", line)
	}
}

func TestRenderRowKeepsDisabledMarkerForSelectedNonK8sNode(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 120
	r := Row{Key: "pod/ns/a/container/0/environment/env/0", Text: "└─ 💬 env API_KEY", Plain: "└─ 💬 env API_KEY"}
	m.selected[0][r.Key] = true

	line := m.renderRow(r, false)
	if strings.Contains(line, "[-]") || strings.Contains(line, "[x]") {
		t.Fatalf("expected selected non-k8s row to hide selector marker, got %q", line)
	}
}

func TestFileSearchFindsMatchesAndAdvances(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.view = &View{Kind: FileOutput, Rows: []string{"kind: Pod", "metadata:", "  name: foo"}, Raw: "kind: Pod\nmetadata:\n  name: foo"}
	m.view.SearchMode = true
	m.view.SearchQuery = "name"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := updated.(Model)
	if m1.view.SearchMode {
		t.Fatalf("expected search mode to close after Enter")
	}
	if len(m1.view.MatchRows) == 0 {
		t.Fatalf("expected at least one search match")
	}

	before := m1.view.ActiveMatch
	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m2 := updated.(Model)
	if len(m2.view.MatchRows) > 1 && m2.view.ActiveMatch == before {
		t.Fatalf("expected n to change active match when multiple matches exist")
	}
}

func TestEscInSearchModeClosesSearchNotFile(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.view = viewHelp()
	m.view.SearchMode = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m1 := updated.(Model)
	if m1.view == nil {
		t.Fatalf("expected view to remain open when Esc closes search mode")
	}
	if m1.view.SearchMode {
		t.Fatalf("expected search mode to be closed")
	}
}

func TestCtrlASelectsAllRowsInActivePane(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "a"}, {Key: "b"}, {Key: "c"}}
	m.resources["a"] = &kube.Resource{Key: "a"}
	m.resources["b"] = &kube.Resource{Key: "b"}
	m.resources["c"] = &kube.Resource{Key: "c"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m1 := updated.(Model)
	if len(m1.selected[0]) != 3 {
		t.Fatalf("expected 3 selected rows after Ctrl+A, got %d", len(m1.selected[0]))
	}
}

func TestSpaceDoesNotSelectNonK8sRow(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "pod/ns/a/container/0/environment/env/0", Type: "env"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m1 := updated.(Model)
	if len(m1.selected[0]) != 0 {
		t.Fatalf("expected non-k8s row to remain unselected, got %d selected", len(m1.selected[0]))
	}
}

func TestCtrlASkipsNonK8sRows(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "pod/ns/a", Type: "pod"}, {Key: "pod/ns/a/container/0/environment/env/0", Type: "env"}}
	m.resources["pod/ns/a"] = &kube.Resource{Key: "pod/ns/a", Type: "pod"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m1 := updated.(Model)
	if len(m1.selected[0]) != 1 {
		t.Fatalf("expected Ctrl+A to select only describable rows, got %d", len(m1.selected[0]))
	}
	if !m1.selected[0]["pod/ns/a"] {
		t.Fatalf("expected describable row to be selected")
	}
}

func TestFilterInputModeAndApply(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "pod/ns/a", Type: "pod", Name: "api"}, {Key: "svc/ns/a", Type: "service", Name: "api-svc"}}
	m.allRowsByPane[0] = m.rowsByPane[0]
	m.width = 80
	m.height = 20

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m1 := updated.(Model)
	if !m1.filterMode {
		t.Fatalf("expected f to enable filter mode")
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p', 'o', 'd'}})
	m2 := updated.(Model)
	if m2.filterQuery != "pod" {
		t.Fatalf("expected filter query to update, got %q", m2.filterQuery)
	}
	if len(m2.rowsByPane[0]) != 1 {
		t.Fatalf("expected live filter to reduce rows to 1, got %d", len(m2.rowsByPane[0]))
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(Model)
	if m3.filterMode {
		t.Fatalf("expected Enter to exit filter mode")
	}

	view := m3.View()
	if !strings.Contains(view, "Filter:") {
		t.Fatalf("expected selector view to show filter bar")
	}
}

func TestFilterMatcherWildcardAndNegation(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.allRowsByPane[0] = []Row{
		{Key: "pod/ns/api", Type: "pod", Name: "api"},
		{Key: "pod/ns/worker", Type: "pod", Name: "worker"},
		{Key: "svc/ns/api", Type: "service", Name: "api"},
	}
	m.applyMainFilter()

	m.filterQuery = "pod/* !worker"
	m.applyMainFilter()

	if len(m.rowsByPane[0]) != 1 {
		t.Fatalf("expected 1 row after wildcard+negation filter, got %d", len(m.rowsByPane[0]))
	}
	if m.rowsByPane[0][0].Key != "pod/ns/api" {
		t.Fatalf("expected remaining key pod/ns/api, got %q", m.rowsByPane[0][0].Key)
	}
}

func TestFilterRebuildsAsciiBranchesAfterFiltering(t *testing.T) {
	trees := &kube.Response{
		Nodes: []kube.Resource{},
		Trees: []kube.Tree{
			{
				Key:  "deploy/ns/root",
				Type: "deployment",
				Meta: map[string]any{"name": "root"},
				Children: []*kube.Tree{
					{Key: "pod/ns/a", Type: "pod", Meta: map[string]any{"name": "alpha"}},
					{Key: "pod/ns/b", Type: "pod", Meta: map[string]any{"name": "bravo"}},
				},
			},
		},
	}

	m := newRun(Options{Mode: ModeSelector, Trees: trees})
	m.filterQuery = "bravo"
	m.applyMainFilter()

	if len(m.rowsByPane[0]) != 2 {
		t.Fatalf("expected root + one filtered child row, got %d", len(m.rowsByPane[0]))
	}
	child := m.rowsByPane[0][1]
	if !strings.Contains(child.Text, "└") {
		t.Fatalf("expected filtered child to be re-rendered as last branch, got %q", child.Text)
	}
	if strings.Contains(child.Text, "├") {
		t.Fatalf("expected no sibling branch glyph after filtering, got %q", child.Text)
	}
}

func TestRowWindowStartCentersAroundCursor(t *testing.T) {
	if got := rowWindowStart(30, 8, 11); got != 7 {
		t.Fatalf("expected centered row window start=7, got %d", got)
	}
}

func TestOQuitsAndEnablesOutput(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "pod/ns/api"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m1 := updated.(Model)
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

func TestRenderRowFileedLongNameStaysSingleRow(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 24
	r := Row{Key: "k", Name: "deployment/applikasjonsplattform/ad-explore-webservice-config"}

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
	r := Row{Key: "k", Text: "└─ colored-row", PlainText: "└─ plain-row"}
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
	r := Row{Key: "k", PlainText: "└─ pod short"}
	m.selected[0][r.Key] = true

	row := m.renderRow(r, false)
	if got := lipgloss.Width(row); got != m.width {
		t.Fatalf("expected selected row width %d, got %d", m.width, got)
	}
}

func TestViewHeaderFillsViewportWidth(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 64

	header := m.Header()
	if got := lipgloss.Width(header); got != m.width {
		t.Fatalf("expected header width %d, got %d", m.width, got)
	}
}

func TestViewFooterFillsViewportWidth(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 64
	m.rowsByPane[0] = []Row{{Key: "pod/ns/a", Type: "pod", Name: "a", Status: "Running"}}

	footer := m.Footer()
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

func TestFlattenTreesUsesASCIITreeRows(t *testing.T) {
	trees := &kube.Response{
		Nodes: []kube.Resource{},
		Trees: []kube.Tree{
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

func TestFlattenTreesAddsSeparatorBetweenRoots(t *testing.T) {
	trees := &kube.Response{
		Nodes: []kube.Resource{},
		Trees: []kube.Tree{
			{Key: "deploy/ns/root-a", Type: "deployment", Meta: map[string]any{"name": "a"}},
			{Key: "deploy/ns/root-b", Type: "deployment", Meta: map[string]any{"name": "b"}},
		},
	}

	rows := flattenTrees(trees)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (root + separator + root), got %d", len(rows))
	}
	if !rows[1].Separator {
		t.Fatalf("expected middle row to be separator")
	}
}

func TestNavigationSkipsSeparatorRows(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{
		{Key: "a", Name: "a"},
		{Separator: true},
		{Key: "b", Name: "b"},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m1 := updated.(Model)
	if m1.cursorByPane[0] != 2 {
		t.Fatalf("expected cursor to skip separator row, got %d", m1.cursorByPane[0])
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyUp})
	m2 := updated.(Model)
	if m2.cursorByPane[0] != 0 {
		t.Fatalf("expected cursor to skip separator row on up, got %d", m2.cursorByPane[0])
	}
}

func TestEscClearsSelectionBeforeQuit(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "a"}}
	m.selected[0]["a"] = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m1 := updated.(Model)
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

func TestViewerFileActiveMatchRow(t *testing.T) {
	view := &View{MatchRows: []int{3, 7, 11}, ActiveMatch: 1}
	if got := view.activeMatchRow(); got != 7 {
		t.Fatalf("expected active match row 7, got %d", got)
	}

	view.ActiveMatch = 99
	if got := view.activeMatchRow(); got != -1 {
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
	m.view = &View{Kind: FileOutput, Rows: []string{"abcdefghijklmnopqrstuvwxyz", "b", "c", "d"}, ColScroll: 6, Scroll: 2}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m1 := updated.(Model)
	if m1.view.ColScroll != 0 {
		t.Fatalf("expected Home to move to row start, got %d", m1.view.ColScroll)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m2 := updated.(Model)
	if m2.view.ColScroll == 0 {
		t.Fatalf("expected End to move to row end, got %d", m2.view.ColScroll)
	}
}

func TestFileGAndGNavigation(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.view = &View{Kind: FileOutput, Rows: []string{"a", "b", "c", "d"}, Scroll: 2}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m1 := updated.(Model)
	if m1.view.Scroll != 0 {
		t.Fatalf("expected g to move to top, got %d", m1.view.Scroll)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m2 := updated.(Model)
	if m2.view.Scroll != 3 {
		t.Fatalf("expected G to move to bottom, got %d", m2.view.Scroll)
	}
}

func TestFileScrollDoesNotOvershootVisibleEnd(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.height = 8 // file rows viewport is height-2 => 6, so all 4 rows fit
	m.view = &View{Kind: FileOutput, Rows: []string{"a", "b", "c", "d"}, Scroll: 0}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m1 := updated.(Model)
	if m1.view.Scroll != 0 {
		t.Fatalf("expected Down to stay at 0 when all rows fit, got %d", m1.view.Scroll)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m2 := updated.(Model)
	if m2.view.Scroll != 0 {
		t.Fatalf("expected G to stay at 0 when all rows fit, got %d", m2.view.Scroll)
	}
}

func TestFilePageDownUsesViewportStep(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.height = 8 // file viewport height = 6, page step = 5
	m.view = &View{Kind: FileOutput, Rows: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}, Scroll: 0}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m1 := updated.(Model)
	if m1.view.Scroll != 4 {
		t.Fatalf("expected PgDn to clamp to last visible page at 4, got %d", m1.view.Scroll)
	}
}

func TestTreePageDownUsesVisibleRowsStep(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.height = 10
	m.footerHeight = 1
	m.rowsByPane[0] = []Row{
		{Key: "a"}, {Key: "b"}, {Key: "c"}, {Key: "d"}, {Key: "e"}, {Key: "f"}, {Key: "g"}, {Key: "h"},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m1 := updated.(Model)
	if m1.cursorByPane[0] != 6 {
		t.Fatalf("expected PgDn to move by visible page step to row 6, got %d", m1.cursorByPane[0])
	}
}

func TestFileGutterMarker(t *testing.T) {
	matchRows := []int{2, 5, 8}

	if got := fileGutterMarker(5, matchRows, 8); got != "*" {
		t.Fatalf("expected match marker '*', got %q", got)
	}
	if got := fileGutterMarker(8, matchRows, 8); got != ">" {
		t.Fatalf("expected active marker '>', got %q", got)
	}
	if got := fileGutterMarker(3, matchRows, 8); got != " " {
		t.Fatalf("expected no marker for non-match row, got %q", got)
	}
}

func TestResolveEditorCommandDefaultsToReadOnlyVi(t *testing.T) {
	bin, args := resolveEditCommand("", "/tmp/sample.txt")
	if bin != "vi" {
		t.Fatalf("expected default editor vi, got %q", bin)
	}
	if !containsString(args, "-R") {
		t.Fatalf("expected vi to include -R for read-only mode, args=%v", args)
	}
	if args[len(args)-1] != "/tmp/sample.txt" {
		t.Fatalf("expected view path as last arg, args=%v", args)
	}
}

func TestResolveEditorCommandForNanoUsesViewerMode(t *testing.T) {
	bin, args := resolveEditCommand("nano", "/tmp/sample.txt")
	if bin != "nano" {
		t.Fatalf("expected nano editor, got %q", bin)
	}
	if !containsString(args, "-v") {
		t.Fatalf("expected nano to include -v for view mode, args=%v", args)
	}
}

func TestResolveEditorCommandForUnknownKeepsArgs(t *testing.T) {
	bin, args := resolveEditCommand("cat -n", "/tmp/sample.txt")
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
	m.view = &View{Kind: FileOutput, Rows: []string{"abcdefghijklmnopqrstuvwxyz"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m1 := updated.(Model)
	if m1.view.ColScroll != 4 {
		t.Fatalf("expected right to increase col scroll to 4, got %d", m1.view.ColScroll)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m2 := updated.(Model)
	if m2.view.ColScroll != 0 {
		t.Fatalf("expected left to decrease col scroll to 0, got %d", m2.view.ColScroll)
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m3 := updated.(Model)
	if m3.view.ColScroll != 0 {
		t.Fatalf("expected left at boundary to stay at 0, got %d", m3.view.ColScroll)
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
	m.view = &View{
		Kind:        FileOutput,
		Rows:        []string{"abcdefghijTARGETxyz"},
		SearchQuery: "target",
		ColScroll:   0,
	}

	m.applySearch()
	if len(m.view.MatchRows) != 1 {
		t.Fatalf("expected one match row, got %d", len(m.view.MatchRows))
	}
	if m.view.ColScroll == 0 {
		t.Fatalf("expected horizontal auto-pan to reveal active match")
	}
}

func TestActiveFileMatchKeepsNextMatchVisible(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 20
	m.view = &View{
		Kind:        FileOutput,
		Rows:        []string{"TARGET", "abcdefghijklmnopTARGET"},
		SearchQuery: "target",
		MatchRows:   []int{0, 1},
		ActiveMatch: 0,
		ColScroll:   0,
	}

	// Move to the second match and ensure viewport follows the active match.
	m.view.ActiveMatch = 1
	m.ensureActiveMatchVisible()

	if m.view.ActiveMatch != 1 {
		t.Fatalf("expected active match to move to second row, got %d", m.view.ActiveMatch)
	}
	if m.view.ColScroll == 0 {
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
	m.view = &View{Kind: FileOutput, Rows: []string{"a"}, SearchQuery: "name"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m1 := updated.(Model)
	if !m1.view.SearchMode {
		t.Fatalf("expected slash to enter search mode")
	}
	if m1.view.SearchQuery != "name" {
		t.Fatalf("expected slash to preserve prior query, got %q", m1.view.SearchQuery)
	}
}

func TestCtrlUClearsSearchQuery(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.view = &View{Kind: FileOutput, Rows: []string{"a"}, SearchMode: true, SearchQuery: "status"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m1 := updated.(Model)
	if m1.view.SearchQuery != "" {
		t.Fatalf("expected Ctrl+U to clear search query, got %q", m1.view.SearchQuery)
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

func TestBuildDescribeArgs(t *testing.T) {
	r := Row{Key: "pod/default/nginx", Type: "pod", Name: "nginx"}
	args, title := buildDescribeArgs(r, "ctx-a", "default")

	got := strings.Join(args, " ")
	want := "--context ctx-a describe pod nginx -n default"
	if got != want {
		t.Fatalf("unexpected describe args: got %q want %q", got, want)
	}
	if title != "pod/nginx" {
		t.Fatalf("unexpected title: %q", title)
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
