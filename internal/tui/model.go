package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type model struct{}

func (model) Init() tea.Cmd {
	return nil
}

func (model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return model{}, tea.Quit
		}
	}

	return model{}, nil
}

func (model) View() string {
	return "chansat\n\nA minimal time tracker.\n\nPress q to quit.\n"
}

func Run() error {
	_, err := tea.NewProgram(model{}).Run()
	if err != nil {
		return err
	}
	return nil
}
