package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type RunConfig struct {
	NewModel     func(Options) tea.Model
	ShouldEmit   func(tea.Model) bool
	OutputKeys   func(tea.Model) []string
	FormatOutput func([]string, bool) (string, error)
	Print        func(string)
}

func Run(opts Options) error {
	return RunWithConfig(opts, RunConfig{
		NewModel: func(o Options) tea.Model {
			return newRun(o)
		},
		ShouldEmit: func(model tea.Model) bool {
			fm, ok := model.(Model)
			return ok && fm.emitSelection
		},
		OutputKeys: func(model tea.Model) []string {
			fm, ok := model.(Model)
			if !ok {
				return nil
			}
			return fm.keysForOutput()
		},
		FormatOutput: formatSelectionOutput,
	})
}

func RunWithConfig(opts Options, cfg RunConfig) error {
	if cfg.NewModel == nil {
		return fmt.Errorf("run config missing NewModel")
	}
	if cfg.ShouldEmit == nil {
		return fmt.Errorf("run config missing ShouldEmit")
	}
	if cfg.OutputKeys == nil {
		return fmt.Errorf("run config missing OutputKeys")
	}
	if cfg.FormatOutput == nil {
		return fmt.Errorf("run config missing FormatOutput")
	}
	if cfg.Print == nil {
		cfg.Print = func(s string) {
			fmt.Print(s)
		}
	}

	m := cfg.NewModel(opts)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	restoreCommandContext := setProgramCommandContext(ctx)
	defer restoreCommandContext()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if opts.Mode != ModeSelector {
		return nil
	}
	if !cfg.ShouldEmit(finalModel) {
		return nil
	}

	outputJSON := opts.JSON || opts.OutputJSON
	output, err := cfg.FormatOutput(cfg.OutputKeys(finalModel), outputJSON)
	if err != nil {
		return err
	}
	if output != "" {
		cfg.Print(output)
	}
	return nil
}
