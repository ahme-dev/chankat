package tui

import (
	"context"

	"chansat/internal/storage"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	rates ratesModel
}

func newModel(ctx context.Context, stor *storage.Storage) model {
	return model{
		rates: newRatesModel(ctx, stor),
	}
}

func (m model) Init() tea.Cmd {
	return m.rates.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.rates, cmd = m.rates.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return m.rates.View()
}

func Run(ctx context.Context, stor *storage.Storage) error {
	_, err := tea.NewProgram(newModel(ctx, stor)).Run()
	return err
}
