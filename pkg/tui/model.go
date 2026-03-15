package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	kube "github.com/karloie/kompass/pkg/kube"
)

type Model struct {
	mode      Mode
	context   string
	namespace string

	width  int
	height int

	activePane   int
	activeColumn int

	sourceTrees  *kube.Response
	allRowsByPane [2][]Row
	rowsByPane   [2][]Row
	cursorByPane [2]int
	selected     [2]map[string]bool
	resources    map[string]*kube.Resource

	footerHeight  int
	view          *View
	emitSelection bool
	filterMode    bool
	filterQuery   string
}

func (m Model) Init() tea.Cmd {
	return nil
}

func newRun(opts Options) Model {
	m := Model{
		mode:         opts.Mode,
		context:      opts.Context,
		namespace:    opts.Namespace,
		footerHeight: 1,
	}
	m.selected[0] = map[string]bool{}
	m.selected[1] = map[string]bool{}
	m.resources = map[string]*kube.Resource{}

	if opts.Mode == ModeSelector && opts.Trees != nil {
		m.sourceTrees = opts.Trees
		allRows := flattenTrees(opts.Trees)
		m.allRowsByPane[0] = allRows
		m.allRowsByPane[1] = singleRows(allRows)
		m.rowsByPane[0] = allRows
		m.rowsByPane[1] = m.allRowsByPane[1]
		for k, v := range opts.Trees.NodeMap() {
			m.resources[k] = v
		}
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
	OnWindowSize   func(tea.WindowSizeMsg)
	HasOpenFile    func() bool
	HandleKey      func(tea.KeyMsg) (tea.Model, tea.Cmd)
	HandleMainKey  func(tea.KeyMsg) (tea.Model, tea.Cmd)
	HandleEditDone func(error)
	Current        func() tea.Model
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

	if done, ok := msg.(interface{ doneErr() error }); ok {
		if cfg.HandleEditDone != nil {
			cfg.HandleEditDone(done.doneErr())
		}
	}

	if cfg.Current != nil {
		return cfg.Current(), nil
	}
	return nil, nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return Update(msg, UpdateConfig{
		OnWindowSize: func(v tea.WindowSizeMsg) {
			m.width = v.Width
			m.height = v.Height
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
		HandleEditDone: func(err error) {
			if m.view == nil {
				return
			}
			if err != nil {
				m.view.ActionStatus = "editor failed: " + err.Error()
				return
			}
			m.view.ActionStatus = "editor closed"
		},
		Current: func() tea.Model {
			return m
		},
	})
}
