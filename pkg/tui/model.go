package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	kube "github.com/karloie/kompass/pkg/kube"
)

type Model struct {
	mode            Mode
	context         string
	namespace       string
	reload          ReloadFunc
	refreshInterval time.Duration

	width  int
	height int

	activePane int

	sourceTrees   *kube.Response
	allRowsByPane [2][]Row
	rowsByPane    [2][]Row
	cursorByPane  [2]int
	mainColScroll [2]int
	selected      [2]map[string]bool
	resources     map[string]*kube.Resource

	footerHeight   int
	view           *View
	submode        Submode
	emitSelection  bool
	filterQuery    string
	filterSaved    string
	contextList    listPickerState
	namespaceList  listPickerState
	refreshing     bool
	lastRefresh    time.Time
	refreshError   string
	selectedAction string
}

type Submode int

const (
	SubmodeNone Submode = iota
	SubmodeConfirmQuit
	SubmodeContextList
	SubmodeNamespaceList
	SubmodeFilter
)

type listPickerState struct {
	Options []string
	Index   int
	Loading bool
	Error   string
}

func (m Model) Init() tea.Cmd {
	if m.canAutoRefresh() {
		return m.nextRefreshTick()
	}
	return nil
}

func newRun(opts Options) Model {
	m := Model{
		mode:            opts.Mode,
		context:         opts.Context,
		namespace:       opts.Namespace,
		reload:          opts.Reload,
		refreshInterval: opts.RefreshInterval,
		footerHeight:    1,
	}
	m.selected[0] = map[string]bool{}
	m.selected[1] = map[string]bool{}
	m.resources = map[string]*kube.Resource{}

	if opts.Mode == ModeSelector && opts.Trees != nil {
		m.setTrees(opts.Trees)
	}

	if opts.Mode == ModeDashboard {
		m.allRowsByPane[0] = []Row{
			{Key: "todo/1", Type: "todo", Name: "Review recent requests", Status: "new", Metadata: map[string]any{"owner": "you"}},
			{Key: "todo/2", Type: "todo", Name: "Investigate cache misses", Status: "in-progress", Metadata: map[string]any{"area": "cache"}},
			{Key: "todo/3", Type: "todo", Name: "Verify release artifacts", Status: "new", Metadata: map[string]any{"priority": "high"}},
		}
		m.rowsByPane[0] = m.allRowsByPane[0]
	}

	return m
}

type UpdateConfig struct {
	OnWindowSize  func(tea.WindowSizeMsg)
	HasOpenFile   func() bool
	HandleKey     func(tea.KeyMsg) (tea.Model, tea.Cmd)
	HandleMainKey func(tea.KeyMsg) (tea.Model, tea.Cmd)
	Current       func() tea.Model
}

func Update(msg tea.Msg, cfg UpdateConfig) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		if cfg.OnWindowSize != nil {
			cfg.OnWindowSize(v)
		}
	case tea.KeyMsg:
		if cfg.HasOpenFile != nil && cfg.HasOpenFile() {
			if cfg.HandleKey != nil {
				return cfg.HandleKey(v)
			}
			break
		}
		if cfg.HandleMainKey != nil {
			return cfg.HandleMainKey(v)
		}
	}

	if cfg.Current != nil {
		return cfg.Current(), nil
	}
	return nil, nil
}

type refreshTickMsg struct{}

type refreshResultMsg struct {
	trees *kube.Response
	err   error
}

type scopeListResultMsg struct {
	mode    string
	options []string
	err     error
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case refreshTickMsg:
		if cmd := m.startRefresh(); cmd != nil {
			return m, cmd
		}
	case refreshResultMsg:
		m.refreshing = false
		if v.err == nil && v.trees != nil {
			m.refreshError = ""
			m.lastRefresh = time.Now()
			m.setTrees(v.trees)
			m.refreshOpenView()
		} else if v.err != nil {
			m.refreshError = v.err.Error()
		}
		if m.canAutoRefresh() {
			return m, m.nextRefreshTick()
		}
		return m, nil
	case scopeListResultMsg:
		expected := SubmodeContextList
		state := &m.contextList
		current := strings.TrimSpace(m.context)
		if v.mode == "namespace" {
			expected = SubmodeNamespaceList
			state = &m.namespaceList
			current = strings.TrimSpace(m.namespace)
		}
		if m.submode != expected {
			return m, nil
		}
		state.Loading = false
		if v.err != nil {
			state.Error = v.err.Error()
		} else {
			state.Error = ""
			state.Options = v.options
			if len(state.Options) == 0 && current != "" {
				state.Options = []string{current}
			}
			for i, opt := range state.Options {
				if opt == current {
					state.Index = i
					break
				}
			}
		}
		return m, nil
	}

	return Update(msg, UpdateConfig{
		OnWindowSize: func(v tea.WindowSizeMsg) {
			m.width = v.Width
			m.height = v.Height
			if m.view != nil {
				m.view.ColScroll = clamp(m.view.ColScroll, 0, m.maxColScroll())
			}
			m.mainColScroll[0] = clamp(m.mainColScroll[0], 0, m.maxMainColScrollForPane(0))
			m.mainColScroll[1] = clamp(m.mainColScroll[1], 0, m.maxMainColScrollForPane(1))
		},
		HasOpenFile: func() bool {
			return m.view != nil
		},
		HandleKey: func(v tea.KeyMsg) (tea.Model, tea.Cmd) {
			return m.handleKey(v)
		},
		HandleMainKey: func(v tea.KeyMsg) (tea.Model, tea.Cmd) {
			return m.handleMainKey(v)
		},
		Current: func() tea.Model {
			return m
		},
	})
}

func (m *Model) startRefresh() tea.Cmd {
	if !m.canAutoRefresh() || m.refreshing {
		return nil
	}
	m.refreshing = true
	m.refreshError = ""
	return m.reloadTreesCmd()
}

func (m Model) canAutoRefresh() bool {
	return m.mode == ModeSelector && m.reload != nil && m.refreshInterval > 0
}

func (m Model) nextRefreshTick() tea.Cmd {
	return tea.Tick(m.refreshInterval, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m Model) reloadTreesCmd() tea.Cmd {
	reload := m.reload
	return func() tea.Msg {
		trees, err := reload()
		return refreshResultMsg{trees: trees, err: err}
	}
}

func (m *Model) setTrees(trees *kube.Response) {
	m.sourceTrees = trees
	m.resources = map[string]*kube.Resource{}
	if trees == nil {
		m.allRowsByPane[0] = nil
		m.allRowsByPane[1] = nil
		m.rowsByPane[0] = nil
		m.rowsByPane[1] = nil
		m.cursorByPane[0] = 0
		m.cursorByPane[1] = 0
		return
	}

	allRows := flattenTrees(trees)
	m.allRowsByPane[0] = allRows
	m.allRowsByPane[1] = singleRows(allRows)
	for k, v := range trees.NodeMap() {
		m.resources[k] = v
	}
	m.applyMainFilter()
}

func (m *Model) refreshOpenView() {
	if m.view == nil || m.view.Kind != FileOutput || len(m.view.Pages) == 0 {
		return
	}
	m.view.syncActivePage()
	rowsHeight := m.viewRowsHeight()
	target := m.view.Target
	if strings.TrimSpace(target.ResourceType) == "" {
		return
	}

	activePage := m.view.pageName()
	previousByName := make(map[string]ViewPage, len(m.view.Pages))
	for _, page := range m.view.Pages {
		previousByName[page.Name] = page
	}

	next := viewResourceFromTarget(target, m.context)
	for i := range next.Pages {
		if prev, ok := previousByName[next.Pages[i].Name]; ok {
			next.Pages[i].Scroll = prev.Scroll
			if next.Pages[i].Name == "logs" {
				prevMax := maxInt(0, len(prev.Rows)-rowsHeight)
				if prev.Scroll == prevMax {
					nextMax := maxInt(0, len(next.Pages[i].Rows)-rowsHeight)
					next.Pages[i].Scroll = nextMax
				}
			}
			next.Pages[i].ColScroll = prev.ColScroll
		}
	}
	for i, page := range next.Pages {
		if page.Name == activePage {
			next.ActivePage = i
			break
		}
	}
	next.syncFromActivePage()
	m.view = next
}

func (m Model) refreshStatusText() string {
	if !m.canAutoRefresh() {
		return ""
	}
	if m.refreshing {
		return "syncing"
	}
	if m.refreshError != "" {
		return "refresh failed: " + m.refreshError
	}
	if !m.lastRefresh.IsZero() {
		return "synced " + m.lastRefresh.Format("15:04:05")
	}
	return "auto-refresh enabled"
}
