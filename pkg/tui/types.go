package tui

import (
	"time"

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
	JSON       bool
	Plain      bool
	FancyOn    bool
	FancyOff   bool
}

type Row struct {
	Key       string
	Type      string
	Name      string
	Text      string
	Plain     string
	PlainText string
	Status    string
	Metadata  map[string]any
	Depth     int
}

type Kind string

const (
	FileYAML Kind = "yaml"
	FileHelp Kind = "help"
)

type View struct {
	Kind      Kind
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

func (m editorDoneMsg) doneErr() error {
	return m.err
}

type rowRenderState struct {
	Focused  bool
	Selected bool
}

const (
	navDoubleTapMin = 120 * time.Millisecond
	navDoubleTapMax = 240 * time.Millisecond
	navJumpDivisor  = 2
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
