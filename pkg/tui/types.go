package tui

import (
	"time"

	"github.com/charmbracelet/lipgloss"
	kube "github.com/karloie/kompass/pkg/kube"
)

type ReloadFunc func() (*kube.Response, error)

type Mode int

const (
	ModeSelector Mode = iota
	ModeDashboard
)

type Options struct {
	Mode            Mode
	Trees           *kube.Response
	Reload          ReloadFunc
	RefreshInterval time.Duration
	Context         string
	Namespace       string
	OutputJSON      bool
	JSON            bool
	Plain           bool
	FancyOn         bool
	FancyOff        bool
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
	Separator bool
}

type Kind string

const (
	FileOutput Kind = "output"
	FileHelp   Kind = "help"
)

type View struct {
	Kind      Kind
	Title     string
	Rows      []string
	Raw       string
	Scroll    int
	ColScroll int

	SearchMode   bool
	SearchQuery  string
	MatchRows    []int
	ActiveMatch  int
	ActionStatus string
}

type editDoneMsg struct {
	err error
}

func (m editDoneMsg) doneErr() error {
	return m.err
}

type rowState struct {
	Focused     bool
	Selected    bool
	Describable bool
}

var (
	accentForeground         = lipgloss.Color("230")
	accentBackground         = lipgloss.Color("24")
	headerStyle              = lipgloss.NewStyle().Bold(true).Foreground(accentForeground).Background(accentBackground).Padding(0, 1)
	footerStyle              = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("238")).Padding(0, 1)
	refreshStatusStyle       = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("246")).Background(lipgloss.Color("238"))
	fileedCell               = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("31"))
	disabledFocusedRowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("238"))
	selectedRowStyle         = lipgloss.NewStyle().Bold(true).Foreground(accentForeground).Background(accentBackground)
	disabledSelectedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("238"))
	matchRowStyle            = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	activeMatchStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("166"))
	rowNumberStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	gutterMatchStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	gutterActiveStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("166"))
	termMatchStyle           = lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("227"))
	termActiveStyle          = lipgloss.NewStyle().Underline(true).Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("166"))
	unselectedMarker         = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("[ ]")
	disabledMarker           = "[-]"
)
