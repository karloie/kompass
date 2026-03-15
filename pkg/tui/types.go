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
	Key        string
	Type       string
	Name       string
	Text       string
	Plain      string
	PlainText  string
	SearchText string
	Status     string
	Metadata   map[string]any
	Depth      int
	Separator  bool
}

type Kind string

const (
	FileOutput Kind = "output"
	FileHelp   Kind = "help"
)

type View struct {
	Kind         Kind
	ResourceName string
	Target       resourceTarget
	Title        string
	Rows         []string
	Raw          string
	Scroll       int
	ColScroll    int

	Pages      []ViewPage
	ActivePage int
}

type ViewPage struct {
	Name      string
	Kind      Kind
	Title     string
	Rows      []string
	Raw       string
	Scroll    int
	ColScroll int
}

func newPagedView(pages []ViewPage) *View {
	v := &View{Pages: pages}
	v.syncFromActivePage()
	return v
}

func (v *View) currentPage() *ViewPage {
	if v == nil || len(v.Pages) == 0 {
		return nil
	}
	idx := clamp(v.ActivePage, 0, len(v.Pages)-1)
	return &v.Pages[idx]
}

func (v *View) pageCount() int {
	if v == nil || len(v.Pages) == 0 {
		return 1
	}
	return len(v.Pages)
}

func (v *View) hasMultiplePages() bool {
	return v != nil && len(v.Pages) > 1
}

func (v *View) pageName() string {
	if page := v.currentPage(); page != nil {
		return page.Name
	}
	return ""
}

func (v *View) cyclePage(step int) {
	if !v.hasMultiplePages() {
		return
	}
	if step == 0 {
		step = 1
	}
	v.syncActivePage()
	next := (v.ActivePage + step) % len(v.Pages)
	if next < 0 {
		next += len(v.Pages)
	}
	v.ActivePage = next
	v.syncFromActivePage()
}

func (v *View) syncActivePage() {
	page := v.currentPage()
	if page == nil {
		return
	}
	page.Kind = v.Kind
	page.Title = v.Title
	page.Rows = v.Rows
	page.Raw = v.Raw
	page.Scroll = v.Scroll
	page.ColScroll = v.ColScroll
}

func (v *View) syncFromActivePage() {
	page := v.currentPage()
	if page == nil {
		return
	}
	v.Kind = page.Kind
	v.Title = page.Title
	v.Rows = page.Rows
	v.Raw = page.Raw
	v.Scroll = page.Scroll
	v.ColScroll = page.ColScroll
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
	commandBarStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("245")).Padding(0, 1)
	activeHeaderTabStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("15"))
	refreshStatusStyle       = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("246")).Background(lipgloss.Color("238"))
	modalTitleStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("250")).Padding(0, 1)
	modalBodyStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("238")).Padding(0, 1)
	modalHintStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("240")).Padding(0, 1)
	focusedCellStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("31"))
	disabledFocusedRowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("238"))
	selectedRowStyle         = lipgloss.NewStyle().Bold(true).Foreground(accentForeground).Background(accentBackground)
	disabledSelectedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("238"))
	rowMetadataStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	rowContinuationStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Bold(true)
	rowNumberStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	unselectedMarker         = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("[ ]")
	disabledMarker           = "[-]"
)
