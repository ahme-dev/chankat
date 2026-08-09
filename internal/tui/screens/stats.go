package screens

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"chankat/internal/storage"
	"chankat/internal/tui/components"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type Stats struct {
	ctx      context.Context
	stor     *storage.Storage
	period   storage.Period
	now      time.Time
	projects []storage.Project
	tasks    []storage.Task
	rates    []storage.Rate
	entries  []storage.Entry
	payments []storage.Payment
	summary  storage.DashboardSummary
	list     list.Model
	detail   bool
	detailID int
	loading  bool
	err      error
	spinner  spinner.Model
	width    int
	height   int
	form     *huh.Form
	from     string
	to       string
}

type statsLoadedMsg struct {
	projects []storage.Project
	tasks    []storage.Task
	rates    []storage.Rate
	entries  []storage.Entry
	payments []storage.Payment
}

type statsFailedMsg struct{ err error }
type statsTickMsg time.Time

type statsProjectItem struct {
	project storage.DashboardProjectSummary
}

func (i statsProjectItem) Title() string { return i.project.ProjectName }
func (i statsProjectItem) Description() string {
	description := components.FormatDuration(i.project.Tracked) + " tracked"
	if amounts := formatStatsAmounts(i.project.EarnedMinor); amounts != "" {
		description += " · " + amounts + " earned"
	}
	if amounts := formatStatsAmounts(i.project.PaidMinor); amounts != "" {
		description += " · " + amounts + " paid"
	}
	return description
}
func (i statsProjectItem) FilterValue() string { return i.project.ProjectName }

type statsTaskItem struct{ task storage.DashboardTaskSummary }

func (i statsTaskItem) Title() string { return i.task.TaskName }
func (i statsTaskItem) Description() string {
	description := components.FormatDuration(i.task.Tracked) + " tracked"
	if amounts := formatStatsAmounts(i.task.EarnedMinor); amounts != "" {
		description += " · " + amounts + " earned"
	}
	return description
}
func (i statsTaskItem) FilterValue() string { return i.task.TaskName }

func NewStats(ctx context.Context, stor *storage.Storage) Stats {
	now := time.Now()
	period, _ := storage.CurrentPeriod(storage.Day, now)
	return Stats{
		ctx: ctx, stor: stor, period: period, now: now, loading: true,
		spinner: spinner.New(), list: newStatsList(),
	}
}

func newStatsList() list.Model {
	model := list.New(nil, components.NewListDelegate(), 0, 0)
	model.SetShowTitle(false)
	model.Styles.FilterCursor = model.Styles.FilterCursor.
		Foreground(components.AccentColor)
	return model
}

func (m Stats) Init() tea.Cmd {
	return tea.Batch(loadStats(m.ctx, m.stor), tickStats(), m.spinner.Tick)
}

func (m Stats) Update(msg tea.Msg) (Stats, tea.Cmd) {
	if m.form != nil {
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
			m.form = nil
			return m, nil
		}
		updated, cmd := m.form.Update(msg)
		m.form = updated.(*huh.Form)
		if m.form.State == huh.StateAborted {
			m.form = nil
			return m, nil
		}
		if m.form.State == huh.StateCompleted {
			var refreshCmd tea.Cmd
			start, startErr := components.ParseDate(m.from)
			end, endErr := components.ParseDate(m.to)
			if startErr == nil && endErr == nil {
				period, err := storage.CustomPeriod(start, end)
				if err == nil {
					m.period = period
					m.detail = false
					refreshCmd = m.refresh()
				}
			}
			m.form = nil
			return m, tea.Batch(cmd, refreshCmd)
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeList()
	case statsLoadedMsg:
		m.projects, m.tasks, m.rates = msg.projects, msg.tasks, msg.rates
		m.entries, m.payments = msg.entries, msg.payments
		m.loading = false
		m.err = nil
		m.updateNow(time.Now())
		return m, m.refresh()
	case statsFailedMsg:
		m.loading = false
		m.err = msg.err
	case statsTickMsg:
		m.updateNow(time.Time(msg))
		return m, tea.Batch(m.refresh(), tickStats())
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "d":
			return m, m.setPeriod(storage.Day)
		case "w":
			return m, m.setPeriod(storage.Week)
		case "m":
			return m, m.setPeriod(storage.Month)
		case "a":
			return m, m.setPeriod(storage.All)
		case "[":
			m.period = storage.MovePeriod(m.period, -1)
			m.detail = false
			return m, m.refresh()
		case "]":
			return m, m.moveForward()
		case "c":
			return m.openCustomPeriod()
		case "enter":
			if item, ok := m.list.SelectedItem().(statsProjectItem); ok {
				m.detail = true
				m.detailID = dashboardProjectID(item.project)
				m.list.ResetFilter()
				return m, m.refreshItems()
			}
			return m, nil
		case "esc":
			if m.detail {
				m.detail = false
				m.list.ResetFilter()
				cmd := m.refreshItems()
				m.selectProject(m.detailID)
				return m, cmd
			}
		case "r":
			m.loading = true
			return m, loadStats(m.ctx, m.stor)
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	case tea.MouseMsg:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	default:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Stats) View() string {
	if m.form != nil {
		return "dashboard / custom period\n\n" + m.form.View() + "\n\n[esc] back"
	}
	if m.loading {
		return m.spinner.View() + " Loading dashboard..."
	}
	if m.err != nil {
		return "Error: " + m.err.Error() + ". Press 'r' to retry."
	}
	return m.headerView() + "\n\n" + m.list.View()
}

func (m Stats) headerView() string {
	var b strings.Builder
	b.WriteString(m.period.Label())
	b.WriteString("\n")
	b.WriteString(components.FormatDuration(m.summary.Tracked))
	b.WriteString(" tracked")
	for _, currency := range dashboardCurrencies(
		m.summary.EarnedMinor, m.summary.PaidMinor, m.summary.NetMinor,
	) {
		fmt.Fprintf(&b, "\n%s earned · %s paid · %s net",
			components.FormatMoney(m.summary.EarnedMinor[currency], currency),
			components.FormatMoney(m.summary.PaidMinor[currency], currency),
			components.FormatMoney(m.summary.NetMinor[currency], currency))
	}
	if m.detail {
		for _, project := range m.summary.Projects {
			if dashboardProjectID(project) == m.detailID {
				b.WriteString("\n\n" + project.ProjectName + " / tasks")
				break
			}
		}
	}
	return b.String()
}

func (m *Stats) refresh() tea.Cmd {
	m.summary = storage.SummarizeDashboard(
		m.projects, m.tasks, m.rates, m.entries, m.payments,
		m.period.Start, m.period.End, m.now,
	)
	return m.refreshItems()
}

func (m *Stats) refreshItems() tea.Cmd {
	if m.detail {
		for _, project := range m.summary.Projects {
			if dashboardProjectID(project) != m.detailID {
				continue
			}
			items := make([]list.Item, len(project.Tasks))
			for i, task := range project.Tasks {
				items[i] = statsTaskItem{task}
			}
			cmd := m.setListItems(items)
			m.resizeList()
			return cmd
		}
		m.detail = false
	}
	items := make([]list.Item, len(m.summary.Projects))
	for i, project := range m.summary.Projects {
		items[i] = statsProjectItem{project}
	}
	cmd := m.setListItems(items)
	m.resizeList()
	return cmd
}

func (m *Stats) setListItems(items []list.Item) tea.Cmd {
	selected := m.list.Index()
	cmd := m.list.SetItems(items)
	if len(items) == 0 {
		return cmd
	}
	if selected >= len(items) {
		selected = len(items) - 1
	}
	if selected < 0 {
		selected = 0
	}
	m.list.Select(selected)
	return cmd
}

func (m *Stats) selectProject(projectID int) {
	for i, item := range m.list.Items() {
		project, ok := item.(statsProjectItem)
		if ok && dashboardProjectID(project.project) == projectID {
			m.list.Select(i)
			return
		}
	}
}

func (m *Stats) resizeList() {
	height := m.height - lipgloss.Height(m.headerView()) - 2
	if height < 1 {
		height = 1
	}
	m.list.SetSize(m.width, height)
}

func dashboardProjectID(project storage.DashboardProjectSummary) int {
	if project.ProjectID == nil {
		return 0
	}
	return *project.ProjectID
}

func (m *Stats) updateNow(now time.Time) {
	followsCurrent := false
	if m.period.Kind != "" && m.period.Kind != storage.All {
		current, err := storage.CurrentPeriod(m.period.Kind, m.now)
		followsCurrent = err == nil &&
			m.period.Start.Equal(current.Start) && m.period.End.Equal(current.End)
	}
	m.now = now
	if followsCurrent {
		current, err := storage.CurrentPeriod(m.period.Kind, now)
		if err == nil {
			m.period = current
		}
	}
}

func (m *Stats) setPeriod(kind storage.PeriodKind) tea.Cmd {
	period, err := storage.CurrentPeriod(kind, m.now)
	if err == nil {
		m.period = period
		m.detail = false
		return m.refresh()
	}
	return nil
}

func (m *Stats) moveForward() tea.Cmd {
	if m.period.Kind == storage.All || m.period.Kind == "" {
		return nil
	}
	current, err := storage.CurrentPeriod(m.period.Kind, m.now)
	if err != nil || !m.period.Start.Before(current.Start) {
		return nil
	}
	m.period = storage.MovePeriod(m.period, 1)
	m.detail = false
	return m.refresh()
}

func (m Stats) openCustomPeriod() (Stats, tea.Cmd) {
	start := m.now
	end := m.now
	if !m.period.Start.IsZero() {
		start = m.period.Start
	}
	if !m.period.End.IsZero() {
		end = m.period.End.AddDate(0, 0, -1)
	}
	m.from = components.FormatDate(start)
	m.to = components.FormatDate(end)
	m.form = huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("From (YYYY-MM-DD)").Value(&m.from).
			Validate(components.Date),
		huh.NewInput().Title("To (YYYY-MM-DD, inclusive)").Value(&m.to).
			Validate(func(value string) error {
				if err := components.Date(value); err != nil {
					return err
				}
				start, err := components.ParseDate(m.from)
				if err != nil {
					return err
				}
				end, err := components.ParseDate(value)
				if err != nil {
					return err
				}
				if end.Before(start) {
					return fmt.Errorf("end date must not precede start date")
				}
				return nil
			}),
	)).WithShowHelp(true).WithWidth(m.width)
	return m, m.form.Init()
}

func (m Stats) FormActive() bool { return m.form != nil }
func (m Stats) GlobalKeysEnabled() bool {
	return m.form == nil && m.list.FilterState() != list.Filtering
}
func (m Stats) Actions() string {
	if m.detail {
		return "[/] search  [esc] projects  [d/w/m/a] period  " +
			"[[/]] move period  [c] custom"
	}
	return "[/] search  [d/w/m/a] period  [[/]] move period  [c] custom  " +
		"[j/k] select  [enter] tasks  [r] reload"
}

func (m *Stats) Reload() tea.Cmd {
	m.loading = true
	return loadStats(m.ctx, m.stor)
}

func loadStats(ctx context.Context, stor *storage.Storage) tea.Cmd {
	return func() tea.Msg {
		projects, err := stor.GetProjects(ctx)
		if err != nil {
			return statsFailedMsg{err}
		}
		tasks, err := stor.GetTasks(ctx)
		if err != nil {
			return statsFailedMsg{err}
		}
		rates, err := stor.GetRates(ctx)
		if err != nil {
			return statsFailedMsg{err}
		}
		entries, err := stor.GetEntries(ctx)
		if err != nil {
			return statsFailedMsg{err}
		}
		payments, err := stor.GetPayments(ctx)
		if err != nil {
			return statsFailedMsg{err}
		}
		return statsLoadedMsg{projects, tasks, rates, entries, payments}
	}
}

func tickStats() tea.Cmd {
	return tea.Tick(time.Second, func(now time.Time) tea.Msg { return statsTickMsg(now) })
}

func dashboardCurrencies(amountMaps ...map[string]int64) []string {
	seen := make(map[string]bool)
	for _, amounts := range amountMaps {
		for currency := range amounts {
			seen[currency] = true
		}
	}
	result := make([]string, 0, len(seen))
	for currency := range seen {
		result = append(result, currency)
	}
	sort.Strings(result)
	return result
}

func formatStatsAmounts(amounts map[string]int64) string {
	currencies := storage.SortedCurrencies(amounts)
	parts := make([]string, len(currencies))
	for i, currency := range currencies {
		parts[i] = components.FormatMoney(amounts[currency], currency)
	}
	return strings.Join(parts, ", ")
}
