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

type uiTheme struct {
	AccentForeground       string
	AccentBackground       string
	FooterForeground       string
	FooterBackground       string
	CommandBarForeground   string
	CommandBarBackground   string
	ActiveTabForeground    string
	ActiveTabBackground    string
	RefreshForeground      string
	RefreshBackground      string
	ModalTitleForeground   string
	ModalTitleBackground   string
	ModalBodyForeground    string
	ModalBodyBackground    string
	ModalHintForeground    string
	ModalHintBackground    string
	FocusedRowForeground   string
	FocusedRowBackground   string
	DisabledRowForeground  string
	DisabledRowBackground  string
	RowMetadataForeground  string
	ContinuationForeground string
	RowNumberForeground    string
	UnselectedMarkerColor  string
}

var defaultUITheme = uiTheme{
	AccentForeground:       "230",
	AccentBackground:       "24",
	FooterForeground:       "252",
	FooterBackground:       "238",
	CommandBarForeground:   "0",
	CommandBarBackground:   "245",
	ActiveTabForeground:    "0",
	ActiveTabBackground:    "15",
	RefreshForeground:      "246",
	RefreshBackground:      "238",
	ModalTitleForeground:   "0",
	ModalTitleBackground:   "250",
	ModalBodyForeground:    "15",
	ModalBodyBackground:    "238",
	ModalHintForeground:    "252",
	ModalHintBackground:    "240",
	FocusedRowForeground:   "229",
	FocusedRowBackground:   "31",
	DisabledRowForeground:  "250",
	DisabledRowBackground:  "238",
	RowMetadataForeground:  "245",
	ContinuationForeground: "243",
	RowNumberForeground:    "245",
	UnselectedMarkerColor:  "245",
}

var currentUITheme = defaultUITheme

var (
	accentForeground         lipgloss.Color
	accentBackground         lipgloss.Color
	headerStyle              lipgloss.Style
	footerStyle              lipgloss.Style
	commandBarStyle          lipgloss.Style
	activeHeaderTabStyle     lipgloss.Style
	refreshStatusStyle       lipgloss.Style
	modalTitleStyle          lipgloss.Style
	modalBodyStyle           lipgloss.Style
	modalHintStyle           lipgloss.Style
	focusedCellStyle         lipgloss.Style
	disabledFocusedRowStyle  lipgloss.Style
	selectedRowStyle         lipgloss.Style
	disabledSelectedRowStyle lipgloss.Style
	rowMetadataStyle         lipgloss.Style
	rowContinuationStyle     lipgloss.Style
	rowNumberStyle           lipgloss.Style
	unselectedMarker         string
	disabledMarker           = "[-]"
)

func applyUITheme(theme uiTheme) {
	currentUITheme = theme

	accentForeground = lipgloss.Color(theme.AccentForeground)
	accentBackground = lipgloss.Color(theme.AccentBackground)
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(accentForeground).Background(accentBackground).Padding(0, 1)
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.FooterForeground)).Background(lipgloss.Color(theme.FooterBackground)).Padding(0, 1)
	commandBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.CommandBarForeground)).Background(lipgloss.Color(theme.CommandBarBackground)).Padding(0, 1)
	activeHeaderTabStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.ActiveTabForeground)).Background(lipgloss.Color(theme.ActiveTabBackground))
	refreshStatusStyle = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color(theme.RefreshForeground)).Background(lipgloss.Color(theme.RefreshBackground))
	modalTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.ModalTitleForeground)).Background(lipgloss.Color(theme.ModalTitleBackground)).Padding(0, 1)
	modalBodyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ModalBodyForeground)).Background(lipgloss.Color(theme.ModalBodyBackground)).Padding(0, 1)
	modalHintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ModalHintForeground)).Background(lipgloss.Color(theme.ModalHintBackground)).Padding(0, 1)
	focusedCellStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.FocusedRowForeground)).Background(lipgloss.Color(theme.FocusedRowBackground))
	disabledFocusedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.DisabledRowForeground)).Background(lipgloss.Color(theme.DisabledRowBackground))
	selectedRowStyle = lipgloss.NewStyle().Bold(true).Foreground(accentForeground).Background(accentBackground)
	disabledSelectedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.DisabledRowForeground)).Background(lipgloss.Color(theme.DisabledRowBackground))
	rowMetadataStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.RowMetadataForeground))
	rowContinuationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ContinuationForeground)).Bold(true)
	rowNumberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.RowNumberForeground))
	unselectedMarker = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.UnselectedMarkerColor)).Render("[ ]")
}
