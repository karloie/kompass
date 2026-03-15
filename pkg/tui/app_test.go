package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	kube "github.com/karloie/kompass/pkg/kube"
)

func stubRunViewCommand(t *testing.T, fn func(name string, args ...string) (string, error)) {
	t.Helper()
	prev := runViewCommand
	runViewCommand = fn
	t.Cleanup(func() {
		runViewCommand = prev
	})
}

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

func TestSelectorRefreshReplacesTreeData(t *testing.T) {
	initial := &kube.Response{
		Nodes: []kube.Resource{{Key: "pod/ns/old", Type: "pod"}},
		Trees: []kube.Tree{{Key: "pod/ns/old", Type: "pod", Meta: map[string]any{"name": "old"}}},
	}
	refreshed := &kube.Response{
		Nodes: []kube.Resource{{Key: "pod/ns/new", Type: "pod"}},
		Trees: []kube.Tree{{Key: "pod/ns/new", Type: "pod", Meta: map[string]any{"name": "new"}}},
	}
	reloadCalls := 0
	m := newRun(Options{
		Mode:            ModeSelector,
		Trees:           initial,
		RefreshInterval: time.Second,
		Reload: func() (*kube.Response, error) {
			reloadCalls++
			return refreshed, nil
		},
	})

	updated, cmd := m.Update(refreshTickMsg{})
	m1 := updated.(Model)
	if cmd == nil {
		t.Fatalf("expected refresh tick to trigger reload command")
	}
	if reloadCalls != 0 {
		t.Fatalf("expected reload to happen inside command, got %d calls before command execution", reloadCalls)
	}

	msg := cmd().(refreshResultMsg)
	if reloadCalls != 1 {
		t.Fatalf("expected one reload call, got %d", reloadCalls)
	}

	updated, nextCmd := m1.Update(msg)
	m2 := updated.(Model)
	if nextCmd == nil {
		t.Fatalf("expected successful refresh to schedule next tick")
	}
	if len(m2.rowsByPane[0]) == 0 || m2.rowsByPane[0][0].Key != "pod/ns/new" {
		t.Fatalf("expected refreshed rows to contain new tree data, got %+v", m2.rowsByPane[0])
	}
	if _, ok := m2.resources["pod/ns/new"]; !ok {
		t.Fatalf("expected refreshed resources map to include new node")
	}
}

func TestManualRefreshKeyStartsReload(t *testing.T) {
	m := newRun(Options{
		Mode:            ModeSelector,
		RefreshInterval: time.Second,
		Reload: func() (*kube.Response, error) {
			return &kube.Response{}, nil
		},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m1 := updated.(Model)
	if cmd == nil {
		t.Fatalf("expected manual refresh key to trigger reload command")
	}
	if !m1.refreshing {
		t.Fatalf("expected manual refresh to mark model as refreshing")
	}
}

func TestRefreshUpdatesOpenResourceView(t *testing.T) {
	phase := "before"
	stubRunViewCommand(t, func(name string, args ...string) (string, error) {
		return phase + ": " + strings.Join(args, " "), nil
	})

	initial := &kube.Response{
		Nodes: []kube.Resource{{Key: "pod/ns/foo", Type: "pod"}},
		Trees: []kube.Tree{{Key: "pod/ns/foo", Type: "pod", Meta: map[string]any{"name": "foo"}}},
	}
	refreshed := &kube.Response{
		Nodes: []kube.Resource{{Key: "pod/ns/foo", Type: "pod"}},
		Trees: []kube.Tree{{Key: "pod/ns/foo", Type: "pod", Meta: map[string]any{"name": "foo"}}},
	}

	m := newRun(Options{Mode: ModeSelector, Trees: initial, Context: "ctx-a", Namespace: "ns", RefreshInterval: time.Second, Reload: func() (*kube.Response, error) {
		return refreshed, nil
	}})
	m.rowsByPane[0] = []Row{{Key: "pod/ns/foo", Type: "pod", Name: "foo", Status: "Running"}}
	m.resources["pod/ns/foo"] = &kube.Resource{Key: "pod/ns/foo", Type: "pod"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := updated.(Model)
	if m1.view == nil {
		t.Fatalf("expected resource view to open")
	}
	if got := strings.Join(m1.view.Rows, "\n"); !strings.Contains(got, "before:") {
		t.Fatalf("expected initial command output, got %q", got)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	m2.view.Scroll = 3
	m2.view.ColScroll = 5

	phase = "after"
	updated, _ = m2.Update(refreshResultMsg{trees: refreshed, err: nil})
	m3 := updated.(Model)

	if m3.view == nil {
		t.Fatalf("expected resource view to remain open")
	}
	if got := strings.Join(m3.view.Rows, "\n"); !strings.Contains(got, "after:") {
		t.Fatalf("expected refreshed command output, got %q", got)
	}
	if m3.view.pageName() != "logs" {
		t.Fatalf("expected active page to remain logs, got %q", m3.view.pageName())
	}
	if m3.view.Scroll != 3 {
		t.Fatalf("expected page scroll preserved, got %d", m3.view.Scroll)
	}
	if m3.view.ColScroll != 5 {
		t.Fatalf("expected page col scroll preserved, got %d", m3.view.ColScroll)
	}
}

func TestRefreshLogsPageFollowsBottom(t *testing.T) {
	phase := "before"
	stubRunViewCommand(t, func(name string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "logs" {
			if phase == "before" {
				return strings.Join([]string{"l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8"}, "\n"), nil
			}
			return strings.Join([]string{"l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10"}, "\n"), nil
		}
		return phase + ": " + strings.Join(args, " "), nil
	})

	initial := &kube.Response{
		Nodes: []kube.Resource{{Key: "pod/ns/foo", Type: "pod"}},
		Trees: []kube.Tree{{Key: "pod/ns/foo", Type: "pod", Meta: map[string]any{"name": "foo"}}},
	}
	refreshed := &kube.Response{
		Nodes: []kube.Resource{{Key: "pod/ns/foo", Type: "pod"}},
		Trees: []kube.Tree{{Key: "pod/ns/foo", Type: "pod", Meta: map[string]any{"name": "foo"}}},
	}

	m := newRun(Options{Mode: ModeSelector, Trees: initial, Context: "ctx-a", Namespace: "ns", RefreshInterval: time.Second, Reload: func() (*kube.Response, error) {
		return refreshed, nil
	}})
	m.height = 8
	m.rowsByPane[0] = []Row{{Key: "pod/ns/foo", Type: "pod", Name: "foo", Status: "Running"}}
	m.resources["pod/ns/foo"] = &kube.Resource{Key: "pod/ns/foo", Type: "pod"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := updated.(Model)
	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.view.pageName() != "logs" {
		t.Fatalf("expected logs page active, got %q", m2.view.pageName())
	}

	// rowsHeight=5 when height=8 in file view, so bottom for 8 lines is 3.
	m2.view.Scroll = 3

	phase = "after"
	updated, _ = m2.Update(refreshResultMsg{trees: refreshed, err: nil})
	m3 := updated.(Model)
	if m3.view.pageName() != "logs" {
		t.Fatalf("expected logs page to remain active, got %q", m3.view.pageName())
	}
	// rowsHeight=5, bottom for 10 lines should be 5.
	if m3.view.Scroll != 5 {
		t.Fatalf("expected logs page to follow new bottom (5), got %d", m3.view.Scroll)
	}
}

func TestFooterIncludesRefreshStatus(t *testing.T) {
	m := newRun(Options{
		Mode:            ModeSelector,
		RefreshInterval: time.Second,
		Reload: func() (*kube.Response, error) {
			return &kube.Response{}, nil
		},
	})
	m.width = 80
	m.rowsByPane[0] = []Row{{Key: "pod/ns/a", Type: "pod", Name: "a", Status: "Running"}}
	m.refreshing = true

	footer := m.Footer()
	if !strings.Contains(footer, "syncing") {
		t.Fatalf("expected footer to include refresh status, got %q", footer)
	}
}

func TestViewDescribeIncludesCommand(t *testing.T) {
	r := Row{Key: "pod/ns/foo", Type: "pod", Name: "foo", Status: "Running", Metadata: map[string]any{"name": "foo"}}
	view := viewDescribe(r, "ctx-a", "ns")
	if view == nil {
		t.Fatalf("expected describe view")
	}
	if view.Title != "kubectl describe pod foo -n ns --context ctx-a" {
		t.Fatalf("expected command in title, got %q", view.Title)
	}
	joined := strings.Join(view.Rows, "\n")
	if strings.Contains(joined, "$ kubectl describe pod foo -n ns --context ctx-a") {
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

	updated, cmd = m1.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(Model)
	if cmd != nil {
		t.Fatalf("expected second Esc to open quit confirmation, not quit immediately")
	}
	if !m2.confirmQuit {
		t.Fatalf("expected second Esc to enable quit confirmation")
	}
}

func TestDoubleEnterReturnsToSelector(t *testing.T) {
	stubRunViewCommand(t, func(name string, args ...string) (string, error) {
		return "describe output", nil
	})

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

func TestTabCyclesInspectPages(t *testing.T) {
	stubRunViewCommand(t, func(name string, args ...string) (string, error) {
		return strings.Join(args, " "), nil
	})

	m := newRun(Options{Mode: ModeSelector, Context: "ctx-a", Namespace: "ns"})
	m.rowsByPane[0] = []Row{{Key: "pod/ns/foo", Type: "pod", Name: "foo", Status: "Running"}}
	m.resources["pod/ns/foo"] = &kube.Resource{Key: "pod/ns/foo", Type: "pod"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := updated.(Model)
	if m1.view == nil {
		t.Fatalf("expected resource view to open")
	}
	if got := len(m1.view.Pages); got != 4 {
		t.Fatalf("expected 4 inspect pages, got %d", got)
	}
	if m1.view.pageName() != "describe" {
		t.Fatalf("expected describe page first, got %q", m1.view.pageName())
	}
	if got := strings.Join(m1.view.Rows, "\n"); !strings.Contains(got, "describe pod foo -n ns --context ctx-a") {
		t.Fatalf("expected describe output on first page, got %q", got)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.view.pageName() != "logs" {
		t.Fatalf("expected logs page after Tab, got %q", m2.view.pageName())
	}
	if got := strings.Join(m2.view.Rows, "\n"); !strings.Contains(got, "logs pod/foo -n ns --context ctx-a") {
		t.Fatalf("expected logs output after Tab, got %q", got)
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated.(Model)
	if m3.view.pageName() != "events" {
		t.Fatalf("expected events page after second Tab, got %q", m3.view.pageName())
	}
	if got := strings.Join(m3.view.Rows, "\n"); !strings.Contains(got, "events --for pod/foo -n ns --context ctx-a") {
		t.Fatalf("expected events output after second Tab, got %q", got)
	}

	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyTab})
	m4 := updated.(Model)
	if m4.view.pageName() != "yaml" {
		t.Fatalf("expected yaml page after third Tab, got %q", m4.view.pageName())
	}
	if got := strings.Join(m4.view.Rows, "\n"); !strings.Contains(got, "get pod foo -o yaml -n ns --context ctx-a") {
		t.Fatalf("expected yaml output after third Tab, got %q", got)
	}

	updated, _ = m4.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m5 := updated.(Model)
	if m5.view.pageName() != "events" {
		t.Fatalf("expected Shift+Tab to move back to events, got %q", m5.view.pageName())
	}
}

func TestInspectPageStatePersistsAcrossTabCycle(t *testing.T) {
	stubRunViewCommand(t, func(name string, args ...string) (string, error) {
		return fmt.Sprintf("output for %s", strings.Join(args, " ")), nil
	})

	m := newRun(Options{Mode: ModeSelector, Namespace: "ns"})
	m.rowsByPane[0] = []Row{{Key: "pod/ns/foo", Type: "pod", Name: "foo", Status: "Running"}}
	m.resources["pod/ns/foo"] = &kube.Resource{Key: "pod/ns/foo", Type: "pod"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := updated.(Model)
	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	m2.view.Scroll = 3
	m2.view.ColScroll = 5

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated.(Model)
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m4 := updated.(Model)

	if m4.view.pageName() != "logs" {
		t.Fatalf("expected to return to logs page, got %q", m4.view.pageName())
	}
	if m4.view.Scroll != 3 {
		t.Fatalf("expected logs page scroll to persist, got %d", m4.view.Scroll)
	}
	if m4.view.ColScroll != 5 {
		t.Fatalf("expected logs page column scroll to persist, got %d", m4.view.ColScroll)
	}
}

func TestResourceViewHidesNonApplicablePagesForKind(t *testing.T) {
	stubRunViewCommand(t, func(name string, args ...string) (string, error) {
		return strings.Join(args, " "), nil
	})

	m := newRun(Options{Mode: ModeSelector, Context: "ctx-a", Namespace: "ns"})
	m.rowsByPane[0] = []Row{{Key: "service/ns/foo", Type: "service", Name: "foo", Status: "Running"}}
	m.resources["service/ns/foo"] = &kube.Resource{Key: "service/ns/foo", Type: "service"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := updated.(Model)
	if m1.view == nil {
		t.Fatalf("expected resource view to open")
	}
	if got := len(m1.view.Pages); got != 3 {
		t.Fatalf("expected 3 applicable pages for service, got %d", got)
	}
	if got := m1.view.pageTabs(); strings.Contains(got, "logs") {
		t.Fatalf("expected logs tab to be hidden for service, got %q", got)
	}

	if m1.view.pageName() != "describe" {
		t.Fatalf("expected describe page first, got %q", m1.view.pageName())
	}
	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.view.pageName() != "events" {
		t.Fatalf("expected events page after Tab, got %q", m2.view.pageName())
	}
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated.(Model)
	if m3.view.pageName() != "yaml" {
		t.Fatalf("expected yaml page after second Tab, got %q", m3.view.pageName())
	}
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyTab})
	m4 := updated.(Model)
	if m4.view.pageName() != "describe" {
		t.Fatalf("expected Tab to wrap back to describe, got %q", m4.view.pageName())
	}
}

func TestInspectHeaderShowsActivePage(t *testing.T) {
	stubRunViewCommand(t, func(name string, args ...string) (string, error) {
		return strings.Join(args, " "), nil
	})

	m := newRun(Options{Mode: ModeSelector, Namespace: "ns"})
	m.width = 120
	m.height = 20
	m.rowsByPane[0] = []Row{{Key: "pod/ns/foo", Type: "pod", Name: "foo", Status: "Running"}}
	m.resources["pod/ns/foo"] = &kube.Resource{Key: "pod/ns/foo", Type: "pod"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m1 := updated.(Model)
	header := m1.headerText()
	if !strings.Contains(header, "foo") || !strings.Contains(header, "describe") || !strings.Contains(header, "logs") || !strings.Contains(header, "events") || !strings.Contains(header, "yaml") {
		t.Fatalf("expected header to show resource tabs, got %q", header)
	}
	if m1.view.pageName() != "describe" {
		t.Fatalf("expected describe to be active page, got %q", m1.view.pageName())
	}
}

func TestFileFooterOmitsEscShortcut(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.view = &View{Kind: FileOutput, Rows: []string{"a"}, Raw: "a"}

	footer := m.footerText()
	if strings.Contains(footer, "Esc") {
		t.Fatalf("expected bottom menu to omit Esc shortcut, got %q", footer)
	}
}

func TestFileLineStatRendersInBottomFooter(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 100
	m.height = 8
	m.view = &View{Kind: FileOutput, Title: "kubectl describe pod foo", Rows: []string{"a", "b", "c"}, Raw: "a\nb\nc", Scroll: 1, ColScroll: 2}

	out := m.toString()
	lines := strings.Split(out, "\n")
	if len(lines) == 0 {
		t.Fatalf("expected rendered output")
	}

	header := lines[0]
	if strings.Contains(header, "line 2/3 col 3") {
		t.Fatalf("expected line stat removed from header, got %q", header)
	}
	if strings.Contains(header, "kubectl describe pod foo") {
		t.Fatalf("expected kubectl command moved out of header, got %q", header)
	}

	command := lines[1]
	if !strings.Contains(command, "kubectl describe pod foo") {
		t.Fatalf("expected command in second line, got %q", command)
	}

	footer := lines[len(lines)-1]
	if !strings.Contains(footer, "line 2/3 col 3") {
		t.Fatalf("expected line stat in footer, got %q", footer)
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
	if m3.filterQuery != "pod" {
		t.Fatalf("expected filter query to remain applied, got %q", m3.filterQuery)
	}
}

func TestFilterModalEscRestoresPreviousQuery(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "pod/ns/a", Type: "pod", Name: "api"}, {Key: "svc/ns/a", Type: "service", Name: "api-svc"}}
	m.allRowsByPane[0] = m.rowsByPane[0]
	m.filterQuery = "svc"
	m.applyMainFilter()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m1 := updated.(Model)
	if !m1.filterMode {
		t.Fatalf("expected / to enable filter mode")
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m2 := updated.(Model)
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p', 'o', 'd'}})
	m3 := updated.(Model)
	if m3.filterQuery != "pod" {
		t.Fatalf("expected live draft query, got %q", m3.filterQuery)
	}

	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m4 := updated.(Model)
	if m4.filterMode {
		t.Fatalf("expected Esc to close filter modal")
	}
	if m4.filterQuery != "svc" {
		t.Fatalf("expected Esc to restore previous query, got %q", m4.filterQuery)
	}
	if len(m4.rowsByPane[0]) != 1 || m4.rowsByPane[0][0].Key != "svc/ns/a" {
		t.Fatalf("expected restored filter results, got %+v", m4.rowsByPane[0])
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

func TestRenderRowFocusedLongNameStaysSingleRow(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 24
	r := Row{Key: "k", Name: "deployment/applikasjonsplattform/ad-explore-webservice-config"}

	row := m.renderRow(r, true)
	if strings.Contains(row, "\n") {
		t.Fatalf("expected focused row to render as a single row")
	}
	if got := lipgloss.Width(row); got != m.width {
		t.Fatalf("expected focused row to invert full width %d, got %d", m.width, got)
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

func TestRenderRowMainUsesPlainContentForPan(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 40
	r := Row{Key: "k", Text: "\x1b[31m└─ colored-row\x1b[0m", Plain: "└─ plain-row"}

	row := m.renderRow(r, false)
	if strings.Contains(row, "colored-row") {
		t.Fatalf("expected unfocused main row to avoid ANSI-colored source content, got %q", row)
	}
	if !strings.Contains(row, "plain-row") {
		t.Fatalf("expected unfocused main row to include plain content, got %q", row)
	}
}

func TestColorizeMainVisibleSegmentMetadataPreservesText(t *testing.T) {
	input := "service ad-explore {clusterIP=10.10.0.1, namespace=applikasjon}"
	styled := colorizeMainVisibleSegment(input, false)
	if plain := ansiEscapePattern.ReplaceAllString(styled, ""); plain != input {
		t.Fatalf("expected colorization to preserve plain text, got %q want %q", plain, input)
	}
}

func TestColorizeMainVisibleSegmentContinuationPreservesText(t *testing.T) {
	input := "resource line ...~"
	styled := colorizeMainVisibleSegment(input, false)
	if plain := ansiEscapePattern.ReplaceAllString(styled, ""); plain != input {
		t.Fatalf("expected colorization to preserve plain text, got %q want %q", plain, input)
	}
}

func TestColorizeMainVisibleSegmentStartsInsideMetadata(t *testing.T) {
	input := "clusterIP=10.98.248.132, ports=[7474/TCP 7687/TCP]"
	styled := colorizeMainVisibleSegment(input, true)
	if plain := ansiEscapePattern.ReplaceAllString(styled, ""); plain != input {
		t.Fatalf("expected colorization to preserve plain text, got %q want %q", plain, input)
	}
}

func TestMetadataOpenAtOffset(t *testing.T) {
	row := "service ad-explore {clusterIP=10.98.248.132, ports=[7474/TCP]}"
	inside := strings.Index(row, "clusterIP")
	outside := strings.Index(row, "service")
	if !metadataOpenAtOffset(row, inside) {
		t.Fatalf("expected offset inside metadata block to report open metadata")
	}
	if metadataOpenAtOffset(row, outside) {
		t.Fatalf("expected offset before metadata block to report closed metadata")
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

func TestViewFooterWithStatusFillsViewportWidth(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 64
	m.rowsByPane[0] = []Row{{Key: "pod/ns/a", Type: "pod", Name: "a", Status: "Running"}}
	m.refreshing = true

	footer := m.Footer()
	if got := lipgloss.Width(footer); got != m.width {
		t.Fatalf("expected status footer width %d, got %d", m.width, got)
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

func TestEscClearsSelectionBeforeQuitConfirmation(t *testing.T) {
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

	updated, cmd = m1.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(Model)
	if cmd != nil {
		t.Fatalf("expected second Esc to open quit confirmation")
	}
	if !m2.confirmQuit {
		t.Fatalf("expected confirmQuit modal state to be enabled")
	}
}

func TestQuitConfirmationEscCancels(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "a"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m1 := updated.(Model)
	if !m1.confirmQuit {
		t.Fatalf("expected quit confirmation enabled")
	}

	updated, cmd := m1.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(Model)
	if cmd != nil {
		t.Fatalf("expected Esc in confirmation to cancel, not quit")
	}
	if m2.confirmQuit {
		t.Fatalf("expected confirmation to be canceled")
	}
}

func TestQuitConfirmationEnterQuits(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = []Row{{Key: "a"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m1 := updated.(Model)
	if !m1.confirmQuit {
		t.Fatalf("expected quit confirmation enabled")
	}

	_, cmd := m1.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected Enter in confirmation to quit")
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

func TestFileHomeEndNavigation(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.view = &View{Kind: FileOutput, Rows: []string{"a", "b", "c", "d"}, Scroll: 2}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m1 := updated.(Model)
	if m1.view.Scroll != 0 {
		t.Fatalf("expected Home to move to top, got %d", m1.view.Scroll)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m2 := updated.(Model)
	if m2.view.Scroll != 3 {
		t.Fatalf("expected End to move to bottom, got %d", m2.view.Scroll)
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

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m2 := updated.(Model)
	if m2.view.Scroll != 0 {
		t.Fatalf("expected End to stay at 0 when all rows fit, got %d", m2.view.Scroll)
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
	if m1.cursorByPane[0] != 7 {
		t.Fatalf("expected PgDn to move by visible page step to row 7, got %d", m1.cursorByPane[0])
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

func TestHelpHorizontalPanning(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.view = &View{Kind: FileHelp, Rows: []string{"abcdefghijklmnopqrstuvwxyz"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m1 := updated.(Model)
	if m1.view.ColScroll != 4 {
		t.Fatalf("expected right to increase help col scroll to 4, got %d", m1.view.ColScroll)
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m2 := updated.(Model)
	if m2.view.ColScroll != 0 {
		t.Fatalf("expected left to decrease help col scroll to 0, got %d", m2.view.ColScroll)
	}
}

func TestMainHorizontalPanning(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 16
	m.rowsByPane[0] = []Row{{Key: "pod/ns/long", Type: "pod", Name: "name-with-very-long-resource"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m1 := updated.(Model)
	if m1.mainColScroll[0] != 4 {
		t.Fatalf("expected right to increase main col scroll to 4, got %d", m1.mainColScroll[0])
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m2 := updated.(Model)
	if m2.mainColScroll[0] != 0 {
		t.Fatalf("expected left to decrease main col scroll to 0, got %d", m2.mainColScroll[0])
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m3 := updated.(Model)
	if m3.mainColScroll[0] != 0 {
		t.Fatalf("expected left at boundary to stay at 0, got %d", m3.mainColScroll[0])
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
	want := "describe pod nginx -n default --context ctx-a"
	if got != want {
		t.Fatalf("unexpected describe args: got %q want %q", got, want)
	}
	if title != "pod/nginx" {
		t.Fatalf("unexpected title: %q", title)
	}
}
