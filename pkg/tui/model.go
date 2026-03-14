package tui

import (
	"time"

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

	rowsByPane   [2][]Row
	cursorByPane [2]int
	selected     [2]map[string]bool
	resources    map[string]*kube.Resource

	footerHeight  int
	file          *View
	emitSelection bool
	lastNavDir    int
	navRepeat     int
	navLastAt     time.Time
	now           func() time.Time
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
		m.rowsByPane[0] = []Row{
			{Key: "todo/1", Type: "todo", Name: "Review recent requests", Status: "new", Metadata: map[string]any{"owner": "you"}},
			{Key: "todo/2", Type: "todo", Name: "Investigate cache misses", Status: "in-progress", Metadata: map[string]any{"area": "cache"}},
			{Key: "todo/3", Type: "todo", Name: "Verify release artifacts", Status: "new", Metadata: map[string]any{"priority": "high"}},
		}
	}

	return m
}

type UpdateConfig struct {
	OnWindowSize     func(tea.WindowSizeMsg)
	HasOpenFile      func() bool
	HandleFileKey    func(tea.KeyMsg) (tea.Model, tea.Cmd)
	HandleMainKey    func(tea.KeyMsg) (tea.Model, tea.Cmd)
	HandleEditorDone func(error)
	CurrentModel     func() tea.Model
}

func Update(msg tea.Msg, cfg UpdateConfig) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		if cfg.OnWindowSize != nil {
			cfg.OnWindowSize(v)
		}
	case tea.KeyMsg:
		if cfg.HasOpenFile != nil && cfg.HasOpenFile() {
			if cfg.HandleFileKey != nil {
				return cfg.HandleFileKey(v)
			}
			break
		}
		if cfg.HandleMainKey != nil {
			return cfg.HandleMainKey(v)
		}
	}

	if done, ok := msg.(interface{ doneErr() error }); ok {
		if cfg.HandleEditorDone != nil {
			cfg.HandleEditorDone(done.doneErr())
		}
	}

	if cfg.CurrentModel != nil {
		return cfg.CurrentModel(), nil
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
			return m.file != nil
		},
		HandleFileKey: func(v tea.KeyMsg) (tea.Model, tea.Cmd) {
			return m.handleFileKey(v)
		},
		HandleMainKey: func(v tea.KeyMsg) (tea.Model, tea.Cmd) {
			return m.handleMainKey(v)
		},
		HandleEditorDone: func(err error) {
			if m.file == nil {
				return
			}
			if err != nil {
				m.file.ActionStatus = "editor failed: " + err.Error()
				return
			}
			m.file.ActionStatus = "editor closed"
		},
		CurrentModel: func() tea.Model {
			return m
		},
	})
}

func (m Model) navPageStep() int {
	rowsHeight := maxInt(1, m.height-1-m.footerHeight)
	// Keep one row overlap between pages for orientation.
	return maxInt(1, rowsHeight-1)
}
