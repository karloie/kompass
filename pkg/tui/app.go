package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	kube "github.com/karloie/kompass/pkg/kube"
)

type Mode int

const (
	ModeSelector Mode = iota
	ModeDashboard
)

type Options struct {
	Mode       Mode
	Trees      *kube.Trees
	Context    string
	Namespace  string
	OutputJSON bool
	Plain      bool
}

type row struct {
	Key       string
	Type      string
	Name      string
	Text      string
	PlainText string
	Status    string
	Metadata  map[string]any
	Depth     int
}

type fileKind string

const (
	fileYAML fileKind = "yaml"
	fileHelp fileKind = "help"
)

type viewerFile struct {
	Kind      fileKind
	Title     string
	Lines     []string
	Raw       string
	Scroll    int
	ColScroll int

	SearchMode   bool
	SearchQuery  string
	MatchLines   []int
	ActiveMatch  int
	ActionStatus string
}

type editorDoneMsg struct {
	err error
}

type rowRenderState struct {
	focused  bool
	selected bool
}

type model struct {
	mode      Mode
	context   string
	namespace string

	width  int
	height int

	activePane   int
	activeColumn int

	rowsByPane   [2][]row
	cursorByPane [2]int
	selected     [2]map[string]bool
	resources    map[string]*kube.Resource

	footerHeight  int
	file          *viewerFile
	emitSelection bool
	lastNavDir    int
	navRepeat     int
	navLastAt     time.Time
	navAnchorDir  int
	now           func() time.Time
}

const (
	navDoubleTapMin = 120 * time.Millisecond
	navDoubleTapMax = 240 * time.Millisecond
	navJumpRows     = 5
)

var (
	accentForeground  = lipgloss.Color("230")
	accentBackground  = lipgloss.Color("24")
	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(accentForeground).Background(accentBackground).Padding(0, 1)
	footerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("238")).Padding(0, 1)
	fileedCell        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("31"))
	selectedRowStyle  = lipgloss.NewStyle().Bold(true).Foreground(accentForeground).Background(accentBackground)
	matchLineStyle    = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	activeMatchStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("166"))
	lineNumberStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	gutterMatchStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	gutterActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("166"))
	termMatchStyle    = lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("227"))
	termActiveStyle   = lipgloss.NewStyle().Underline(true).Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("166"))
	unselectedMarker  = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("[ ]")
)

func (m model) Init() tea.Cmd {
	return nil
}

func hasArg(args []string, needle string) bool {
	for _, arg := range args {
		if arg == needle {
			return true
		}
	}
	return false
}

func newRun(opts Options) model {
	m := model{
		mode:         opts.Mode,
		context:      opts.Context,
		namespace:    opts.Namespace,
		footerHeight: 1,
		now:          time.Now,
	}
	m.selected[0] = map[string]bool{}
	m.selected[1] = map[string]bool{}
	m.resources = map[string]*kube.Resource{}

	if opts.Mode == ModeSelector && opts.Trees != nil {
		allRows := flattenTrees(opts.Trees)
		m.rowsByPane[0] = allRows
		m.rowsByPane[1] = singleRows(allRows)
		for k, v := range opts.Trees.Nodes {
			m.resources[k] = v
		}
	}

	if opts.Mode == ModeDashboard {
		m.rowsByPane[0] = []row{
			{Key: "todo/1", Type: "todo", Name: "Review recent requests", Status: "new", Metadata: map[string]any{"owner": "you"}},
			{Key: "todo/2", Type: "todo", Name: "Investigate cache misses", Status: "in-progress", Metadata: map[string]any{"area": "cache"}},
			{Key: "todo/3", Type: "todo", Name: "Verify release artifacts", Status: "new", Metadata: map[string]any{"priority": "high"}},
		}
	}

	return m
}

func Run(opts Options) error {
	m := newRun(opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if opts.Mode != ModeSelector {
		return nil
	}
	fm, ok := finalModel.(model)
	if !ok || !fm.emitSelection {
		return nil
	}
	output, err := formatSelectionOutput(fm.keysForOutput(), opts.OutputJSON)
	if err != nil {
		return err
	}
	if output != "" {
		fmt.Print(output)
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if m.file != nil {
			return m.handleFileKey(msg)
		}

		return m.handleMainKey(msg)
	}

	switch msg := msg.(type) {
	case editorDoneMsg:
		if m.file != nil {
			if msg.err != nil {
				m.file.ActionStatus = "editor failed: " + msg.err.Error()
			} else {
				m.file.ActionStatus = "editor closed"
			}
		}
	}
	return m, nil
}

// helpers

func fit(s string, width int) string {
	if width <= 0 {
		return s
	}
	return truncate(s, width)
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return string(r[:1])
	}
	if width <= 3 {
		return string(r[:width])
	}
	return string(r[:width-3]) + "..."
}

func pad(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) >= width {
		return string(r[:width])
	}
	return s + strings.Repeat(" ", width-len(r))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
