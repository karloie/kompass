package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	kube "github.com/karloie/kompass/pkg/kube"
)

func TestTabAndShiftTabSwitchPanes(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
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
	m := newModel(Options{Mode: ModeSelector})
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
	m := newModel(Options{Mode: ModeSelector})
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

func TestQuestionMarkOpensHelpModal(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m1 := updated.(model)
	if m1.modal == nil {
		t.Fatalf("expected help modal to open")
	}
	if m1.modal.Kind != modalHelp {
		t.Fatalf("expected help modal kind, got %q", m1.modal.Kind)
	}
}

func TestOpenYAMLModalPrefersResourceData(t *testing.T) {
	r := row{Key: "pod/ns/foo", Type: "pod", Name: "foo", Status: "Running", Metadata: map[string]any{"name": "foo"}}
	res := map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": "foo"}}
	modal := openYAMLModal(r, &kube.Resource{Resource: res})
	if modal == nil {
		t.Fatalf("expected yaml modal")
	}
	joined := strings.Join(modal.Lines, "\n")
	if !strings.Contains(joined, "kind: Pod") {
		t.Fatalf("expected yaml modal to include resource kind, got:\n%s", joined)
	}
}

func TestEscClosesModalBeforeQuit(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
	m.modal = openHelpModal()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m1 := updated.(model)
	if m1.modal != nil {
		t.Fatalf("expected Esc to close modal first")
	}
	if cmd != nil {
		t.Fatalf("expected no quit command while closing modal")
	}

	_, cmd = m1.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatalf("expected second Esc to quit app")
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

	if got := footerSummary("ctx", "ns", r, 0); !strings.Contains(got, "type=pod") {
		t.Fatalf("expected type summary, got %q", got)
	}
	if got := footerSummary("ctx", "ns", r, 1); !strings.Contains(got, "key=pod/ns/foo") {
		t.Fatalf("expected name summary with key, got %q", got)
	}
	if got := footerSummary("ctx", "ns", r, 2); !strings.Contains(got, "reason=CrashLoopBackOff") {
		t.Fatalf("expected status summary with reason, got %q", got)
	}
	if got := footerSummary("ctx", "ns", r, 3); !strings.Contains(got, "namespace=ns") {
		t.Fatalf("expected metadata summary, got %q", got)
	}
}

func TestModalSearchFindsMatchesAndJumps(t *testing.T) {
	r := row{Key: "k", Type: "pod", Name: "foo"}
	resource := &kube.Resource{Resource: map[string]any{
		"kind": "Pod",
		"metadata": map[string]any{
			"name": "foo",
		},
	}}
	m := newModel(Options{Mode: ModeSelector})
	m.modal = openYAMLModal(r, resource)
	m.modal.SearchMode = true
	m.modal.SearchQuery = "name"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := updated.(model)
	if m1.modal.SearchMode {
		t.Fatalf("expected search mode to close after Enter")
	}
	if len(m1.modal.MatchLines) == 0 {
		t.Fatalf("expected at least one search match")
	}

	before := m1.modal.ActiveMatch
	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m2 := updated.(model)
	if len(m2.modal.MatchLines) > 1 && m2.modal.ActiveMatch == before {
		t.Fatalf("expected n to change active match when multiple matches exist")
	}
}

func TestEscInSearchModeClosesSearchNotModal(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
	m.modal = openHelpModal()
	m.modal.SearchMode = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m1 := updated.(model)
	if m1.modal == nil {
		t.Fatalf("expected modal to remain open when Esc closes search mode")
	}
	if m1.modal.SearchMode {
		t.Fatalf("expected search mode to be closed")
	}
}

func TestCtrlASelectsAllRowsInActivePane(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []row{{Key: "a"}, {Key: "b"}, {Key: "c"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m1 := updated.(model)
	if len(m1.selected[0]) != 3 {
		t.Fatalf("expected 3 selected rows after Ctrl+A, got %d", len(m1.selected[0]))
	}
}

func TestOQuitsAndEnablesOutput(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
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

func TestVOpensSelectionModal(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []row{{Key: "a"}, {Key: "b"}}
	m.selected[0]["a"] = true
	m.selected[0]["b"] = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m1 := updated.(model)
	if m1.modal == nil {
		t.Fatalf("expected v to open selected rows modal")
	}
	if m1.modal.Kind != modalYAML {
		t.Fatalf("expected selected rows modal to be YAML kind, got %q", m1.modal.Kind)
	}
	if !strings.Contains(m1.modal.Raw, "selectedKeys") || !strings.Contains(m1.modal.Raw, "- a") {
		t.Fatalf("expected selected rows modal to include selected keys, got:\n%s", m1.modal.Raw)
	}
}

func TestEscClearsSelectionBeforeQuit(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
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

func TestViewerModalActiveMatchLine(t *testing.T) {
	modal := &viewerModal{MatchLines: []int{3, 7, 11}, ActiveMatch: 1}
	if got := modal.activeMatchLine(); got != 7 {
		t.Fatalf("expected active match line 7, got %d", got)
	}

	modal.ActiveMatch = 99
	if got := modal.activeMatchLine(); got != -1 {
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
	line := "metadata.name: petshop-db"

	if got := highlightSearchTerm(line, "", false); got != line {
		t.Fatalf("expected empty query to keep line unchanged")
	}

	inactive := highlightSearchTerm(line, "name", false)
	active := highlightSearchTerm(line, "name", true)

	if !strings.Contains(inactive, "name") {
		t.Fatalf("expected inactive highlighting to preserve searched term")
	}
	if !strings.Contains(active, "name") {
		t.Fatalf("expected active highlighting to preserve searched term")
	}
	if got := highlightSearchTerm(line, "missing", true); got != line {
		t.Fatalf("expected missing query to keep line unchanged")
	}
}

func TestModalHomeEndNavigation(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
	m.width = 20
	m.modal = &viewerModal{Kind: modalYAML, Lines: []string{"abcdefghijklmnopqrstuvwxyz", "b", "c", "d"}, ColScroll: 6, Scroll: 2}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m1 := updated.(model)
	if m1.modal.ColScroll != 0 {
		t.Fatalf("expected Home to jump to line start, got %d", m1.modal.ColScroll)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m2 := updated.(model)
	if m2.modal.ColScroll == 0 {
		t.Fatalf("expected End to jump to line end, got %d", m2.modal.ColScroll)
	}
}

func TestModalGAndGNavigation(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
	m.modal = &viewerModal{Kind: modalYAML, Lines: []string{"a", "b", "c", "d"}, Scroll: 2}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m1 := updated.(model)
	if m1.modal.Scroll != 0 {
		t.Fatalf("expected g to jump to top, got %d", m1.modal.Scroll)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m2 := updated.(model)
	if m2.modal.Scroll != 3 {
		t.Fatalf("expected G to jump to bottom, got %d", m2.modal.Scroll)
	}
}

func TestModalGutterMarker(t *testing.T) {
	matchLines := []int{2, 5, 8}

	if got := modalGutterMarker(5, matchLines, 8); got != "*" {
		t.Fatalf("expected match marker '*', got %q", got)
	}
	if got := modalGutterMarker(8, matchLines, 8); got != ">" {
		t.Fatalf("expected active marker '>', got %q", got)
	}
	if got := modalGutterMarker(3, matchLines, 8); got != " " {
		t.Fatalf("expected no marker for non-match line, got %q", got)
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

func TestModalHorizontalPanning(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
	m.modal = &viewerModal{Kind: modalYAML, Lines: []string{"abcdefghijklmnopqrstuvwxyz"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m1 := updated.(model)
	if m1.modal.ColScroll != 4 {
		t.Fatalf("expected right to increase col scroll to 4, got %d", m1.modal.ColScroll)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m2 := updated.(model)
	if m2.modal.ColScroll != 0 {
		t.Fatalf("expected left to decrease col scroll to 0, got %d", m2.modal.ColScroll)
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m3 := updated.(model)
	if m3.modal.ColScroll != 0 {
		t.Fatalf("expected left at boundary to stay at 0, got %d", m3.modal.ColScroll)
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

func TestApplyModalSearchKeepsActiveMatchVisible(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
	m.width = 18
	m.modal = &viewerModal{
		Kind:        modalYAML,
		Lines:       []string{"abcdefghijTARGETxyz"},
		SearchQuery: "target",
		ColScroll:   0,
	}

	m.applyModalSearch()
	if len(m.modal.MatchLines) != 1 {
		t.Fatalf("expected one match line, got %d", len(m.modal.MatchLines))
	}
	if m.modal.ColScroll == 0 {
		t.Fatalf("expected horizontal auto-pan to reveal active match")
	}
}

func TestJumpModalMatchKeepsNextMatchVisible(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
	m.width = 20
	m.modal = &viewerModal{
		Kind:        modalYAML,
		Lines:       []string{"TARGET", "abcdefghijklmnopTARGET"},
		SearchQuery: "target",
		MatchLines:  []int{0, 1},
		ActiveMatch: 0,
		ColScroll:   0,
	}

	m.jumpModalMatch(1)
	if m.modal.ActiveMatch != 1 {
		t.Fatalf("expected active match to move to second line, got %d", m.modal.ActiveMatch)
	}
	if m.modal.ColScroll == 0 {
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
	m := newModel(Options{Mode: ModeSelector})
	m.modal = &viewerModal{Kind: modalYAML, Lines: []string{"a"}, SearchQuery: "name"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m1 := updated.(model)
	if !m1.modal.SearchMode {
		t.Fatalf("expected slash to enter search mode")
	}
	if m1.modal.SearchQuery != "name" {
		t.Fatalf("expected slash to preserve prior query, got %q", m1.modal.SearchQuery)
	}
}

func TestCtrlUClearsSearchQuery(t *testing.T) {
	m := newModel(Options{Mode: ModeSelector})
	m.modal = &viewerModal{Kind: modalYAML, Lines: []string{"a"}, SearchMode: true, SearchQuery: "status"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m1 := updated.(model)
	if m1.modal.SearchQuery != "" {
		t.Fatalf("expected Ctrl+U to clear search query, got %q", m1.modal.SearchQuery)
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
