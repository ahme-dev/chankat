package tui

import (
	"context"

	"chansat/internal/storage"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type ratesModel struct {
	ctx     context.Context
	stor    *storage.Storage
	rates   list.Model
	err     error
	loading bool
}

type ratesLoadedMsg []storage.Rate

type ratesFailedMsg struct {
	err error
}

func loadRates(ctx context.Context, stor *storage.Storage) tea.Cmd {
	return func() tea.Msg {
		rates, err := stor.GetRates(ctx)
		if err != nil {
			return ratesFailedMsg{err: err}
		}
		return ratesLoadedMsg(rates)
	}
}

func newRatesModel(ctx context.Context, stor *storage.Storage) ratesModel {
	rates := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	rates.Title = "Rates"

	return ratesModel{
		ctx:     ctx,
		stor:    stor,
		rates:   rates,
		loading: true,
	}
}

func (m ratesModel) Init() tea.Cmd {
	return loadRates(m.ctx, m.stor)
}

func (m ratesModel) Update(msg tea.Msg) (ratesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ratesLoadedMsg:
		m.loading = false
		m.err = nil
		return m, m.rates.SetItems(rateItems(msg))
	case ratesFailedMsg:
		m.loading = false
		m.err = msg.err
	case tea.WindowSizeMsg:
		m.rates.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		switch {
		case msg.String() == "ctrl+c":
			return m, tea.Quit
		case msg.String() == "r" && m.rates.FilterState() == list.Unfiltered:
			m.loading = true
			return m, loadRates(m.ctx, m.stor)
		}
	}

	var cmd tea.Cmd
	m.rates, cmd = m.rates.Update(msg)
	return m, cmd
}

func (m ratesModel) View() string {
	if m.loading {
		return "Loading rates..."
	}
	if m.err != nil {
		return "Error: " + m.err.Error() + ". Press 'r' to retry."
	}
	return m.rates.View()
}
