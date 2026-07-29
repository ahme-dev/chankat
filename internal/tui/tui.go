package tui

import (
	"context"
	"fmt"
	"strings"

	"chankat/internal/storage"
	"chankat/internal/tui/screens"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	tasksScreen screen = iota
	projectsScreen
	ratesScreen
	paymentsScreen
)

type tab struct {
	label  string
	screen screen
}

var tabs = []tab{
	{label: "Tasks", screen: tasksScreen},
	{label: "Projects", screen: projectsScreen},
	{label: "Rates", screen: ratesScreen},
	{label: "Payments", screen: paymentsScreen},
}

var (
	tabStyle       = lipgloss.NewStyle().Padding(0, 1)
	activeTabStyle = tabStyle.Bold(true).Reverse(true)
)

type model struct {
	active    screen
	dashboard screens.Dashboard
	projects  screens.Projects
	rates     screens.Rates
	payments  screens.Payments
	height    int
}

func newModel(ctx context.Context, stor *storage.Storage) model {
	return model{
		active:    tasksScreen,
		dashboard: screens.NewDashboard(ctx, stor),
		projects:  screens.NewProjects(ctx, stor),
		rates:     screens.NewRates(ctx, stor),
		payments:  screens.NewPayments(ctx, stor),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.dashboard.Init(),
		m.projects.Init(),
		m.rates.Init(),
		m.payments.Init(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		msg.Height -= 5
		if msg.Height < 1 {
			msg.Height = 1
		}
		return m.updateAll(msg)
	case tea.MouseMsg:
		mouse := tea.MouseEvent(msg)
		if !m.formActive() &&
			mouse.Y == 0 &&
			mouse.Button == tea.MouseButtonLeft &&
			mouse.Action == tea.MouseActionPress {
			if target, ok := tabAt(mouse.X); ok {
				return m.activate(target)
			}
		}
		return m.updateActive(msg)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" ||
			msg.String() == "q" && !m.formActive() && m.globalKeysEnabled() {
			return m, tea.Quit
		}
		if m.globalKeysEnabled() {
			switch msg.String() {
			case "left", "h":
				return m.activate(adjacentTab(m.active, -1))
			case "right", "l":
				return m.activate(adjacentTab(m.active, 1))
			case "1":
				return m.activate(tasksScreen)
			case "2":
				return m.activate(projectsScreen)
			case "3":
				return m.activate(ratesScreen)
			case "4":
				return m.activate(paymentsScreen)
			}
		}
		return m.updateActive(msg)
	default:
		return m.updateAll(msg)
	}
}

func (m model) updateActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.active {
	case tasksScreen:
		m.dashboard, cmd = m.dashboard.Update(msg)
	case projectsScreen:
		m.projects, cmd = m.projects.Update(msg)
	case ratesScreen:
		m.rates, cmd = m.rates.Update(msg)
	case paymentsScreen:
		m.payments, cmd = m.payments.Update(msg)
	}
	return m, cmd
}

func (m model) updateAll(msg tea.Msg) (tea.Model, tea.Cmd) {
	var commands [4]tea.Cmd
	m.dashboard, commands[0] = m.dashboard.Update(msg)
	m.projects, commands[1] = m.projects.Update(msg)
	m.rates, commands[2] = m.rates.Update(msg)
	m.payments, commands[3] = m.payments.Update(msg)
	return m, tea.Batch(commands[:]...)
}

func (m model) View() string {
	var content string
	switch m.active {
	case tasksScreen:
		content = m.dashboard.View()
	case projectsScreen:
		content = m.projects.View()
	case ratesScreen:
		content = m.rates.View()
	case paymentsScreen:
		content = m.payments.View()
	}

	if m.formActive() {
		return m.fillHeight(content)
	}

	contentHeight := m.height - 5
	if contentHeight < 1 {
		contentHeight = 1
	}
	content = lipgloss.NewStyle().Height(contentHeight).Render(content)
	return fmt.Sprintf(
		"%s\n\n%s\n\n[1-4] tabs  [h/l] cycle\n%s",
		renderTabs(m.active),
		content,
		m.actions(),
	)
}

func (m model) fillHeight(view string) string {
	if m.height < 1 {
		return view
	}
	return lipgloss.NewStyle().Height(m.height).Render(view)
}

func (m model) activate(target screen) (tea.Model, tea.Cmd) {
	m.active = target
	var cmd tea.Cmd
	switch target {
	case tasksScreen:
		cmd = m.dashboard.Reload()
	case projectsScreen:
		cmd = m.projects.Reload()
	case ratesScreen:
		cmd = m.rates.Reload()
	case paymentsScreen:
		cmd = m.payments.Reload()
	}
	return m, cmd
}

func (m model) formActive() bool {
	switch m.active {
	case tasksScreen:
		return m.dashboard.FormActive()
	case projectsScreen:
		return m.projects.FormActive()
	case ratesScreen:
		return m.rates.FormActive()
	case paymentsScreen:
		return m.payments.FormActive()
	default:
		return false
	}
}

func (m model) globalKeysEnabled() bool {
	switch m.active {
	case tasksScreen:
		return m.dashboard.GlobalKeysEnabled()
	case projectsScreen:
		return m.projects.GlobalKeysEnabled()
	case ratesScreen:
		return m.rates.GlobalKeysEnabled()
	case paymentsScreen:
		return m.payments.GlobalKeysEnabled()
	default:
		return true
	}
}

func (m model) actions() string {
	switch m.active {
	case tasksScreen:
		return m.dashboard.Actions()
	case projectsScreen:
		return m.projects.Actions()
	case ratesScreen:
		return m.rates.Actions()
	case paymentsScreen:
		return m.payments.Actions()
	default:
		return ""
	}
}

func renderTabs(active screen) string {
	rendered := make([]string, len(tabs))
	for i, tab := range tabs {
		style := tabStyle
		if tab.screen == active {
			style = activeTabStyle
		}
		rendered[i] = style.Render(tab.label)
	}
	return strings.Join(rendered, "│")
}

func tabAt(x int) (screen, bool) {
	offset := 0
	for _, tab := range tabs {
		width := lipgloss.Width(tabStyle.Render(tab.label))
		if x >= offset && x < offset+width {
			return tab.screen, true
		}
		offset += width + 1
	}
	return 0, false
}

func adjacentTab(active screen, offset int) screen {
	for i, tab := range tabs {
		if tab.screen == active {
			next := (i + offset + len(tabs)) % len(tabs)
			return tabs[next].screen
		}
	}
	return tasksScreen
}

func Run(ctx context.Context, stor *storage.Storage) error {
	_, err := tea.NewProgram(
		newModel(ctx, stor),
		tea.WithMouseCellMotion(),
	).Run()
	return err
}
