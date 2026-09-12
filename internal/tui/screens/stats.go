package screens

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"chankat/internal/storage"
	"chankat/internal/tui/components"

	"github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Stats struct {
	ctx        context.Context
	stor       *storage.Storage
	period     storage.Period
	now        time.Time
	projects   []storage.Project
	tasks      []storage.Task
	rates      []storage.Rate
	entries    []storage.Entry
	payments   []storage.Payment
	summary    storage.DashboardSummary
	list       list.Model
	loading    bool
	err        error
	spinner    spinner.Model
	width      int
	height     int
	periodMenu *components.PeriodMenu
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

type OpenTasksMsg struct {
	ProjectID int
	Period    storage.Period
}

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
	if amounts := formatStatsBalances(i.project.BalanceMinor); amounts != "" {
		description += " · current " + amounts
	}
	return description
}
func (i statsProjectItem) FilterValue() string { return i.project.ProjectName }

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
	if m.periodMenu != nil {
		if tick, ok := msg.(statsTickMsg); ok {
			m.updateNow(time.Time(tick))
			return m, tea.Batch(m.refresh(), tickStats())
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
			m.periodMenu = nil
			return m, nil
		}
		cmd := m.periodMenu.Update(msg)
		if m.periodMenu.Aborted() {
			m.periodMenu = nil
			return m, nil
		}
		if m.periodMenu.Completed() {
			var refreshCmd tea.Cmd
			period, err := m.periodMenu.Period(m.now)
			if err == nil {
				m.period = period
				refreshCmd = m.refresh()
			}
			m.periodMenu = nil
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
			m.resizeList()
			return m, cmd
		}
		switch msg.String() {
		case "f":
			return m.openPeriodMenu()
		case "F":
			return m, m.setPeriod(storage.All)
		case "shift+left", "H":
			m.period = storage.MovePeriod(m.period, -1)
			return m, m.refresh()
		case "shift+right", "L":
			return m, m.moveForward()
		case "shift+up", "K":
			m.period = storage.StepPeriodKind(m.period, -1, m.now)
			return m, m.refresh()
		case "shift+down", "J":
			m.period = storage.StepPeriodKind(m.period, 1, m.now)
			return m, m.refresh()
		case "enter":
			if item, ok := m.list.SelectedItem().(statsProjectItem); ok {
				message := OpenTasksMsg{
					ProjectID: dashboardProjectID(item.project),
					Period:    m.period,
				}
				return m, func() tea.Msg { return message }
			}
			return m, nil
		case "r":
			m.loading = true
			return m, loadStats(m.ctx, m.stor)
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		m.resizeList()
		return m, cmd
	case tea.MouseMsg:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		m.resizeList()
		return m, cmd
	default:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		m.resizeList()
		return m, cmd
	}
	return m, nil
}

func (m Stats) View() string {
	if m.periodMenu != nil {
		return "dashboard / filters\n\n" + m.periodMenu.View() + "\n\n[esc] back"
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
		m.summary.BalanceMinor,
	) {
		fmt.Fprintf(&b, "\n%s earned · %s paid · %s net · current %s",
			components.FormatMoney(m.summary.EarnedMinor[currency], currency),
			components.FormatMoney(m.summary.PaidMinor[currency], currency),
			components.FormatMoney(m.summary.NetMinor[currency], currency),
			components.FormatBalance(m.summary.BalanceMinor[currency], currency))
	}
	if chart := m.timelineChartView(lipgloss.Height(b.String())); chart != "" {
		b.WriteString("\n\nTracked over time by project (hours)\n" + chart)
	}
	return b.String()
}

func (m Stats) timelineChartView(headerHeight int) string {
	if m.width < 45 {
		return ""
	}
	projects := m.visibleChartProjects()
	if len(projects) == 0 {
		return ""
	}
	legend := chartLegend(projects, m.width)
	chartHeight := m.height - headerHeight - lipgloss.Height(legend) - 8
	if chartHeight < 6 {
		return ""
	}
	if chartHeight > 9 {
		chartHeight = 9
	}

	allBuckets := storage.SummarizeDashboardTimeline(
		m.visibleEntries(), m.period, m.now,
	)
	if len(allBuckets) < 2 {
		return ""
	}
	chartPeriod := storage.Period{
		Kind:  m.period.Kind,
		Start: allBuckets[0].Start,
		End:   allBuckets[len(allBuckets)-1].End,
	}
	maxHours := 0.0
	series := make([][]storage.DashboardTimeBucket, len(projects))
	for i, project := range projects {
		series[i] = storage.SummarizeDashboardTimeline(
			m.entriesForProject(dashboardProjectID(project)),
			chartPeriod,
			m.now,
		)
		for _, bucket := range series[i] {
			hours := bucket.Tracked.Hours()
			if hours > maxHours {
				maxHours = hours
			}
		}
	}
	if maxHours == 0 {
		return ""
	}
	maxHours *= 1.1
	if maxHours < 1 {
		maxHours = 1
	}

	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	chart := timeserieslinechart.New(
		m.width,
		chartHeight,
		timeserieslinechart.WithTimeRange(
			allBuckets[0].Start,
			allBuckets[len(allBuckets)-1].End,
		),
		timeserieslinechart.WithYRange(0, maxHours),
		timeserieslinechart.WithXYSteps(4, 2),
		timeserieslinechart.WithXLabelFormatter(timelineLabelFormatter(allBuckets)),
		timeserieslinechart.WithYLabelFormatter(func(_ int, value float64) string {
			return fmt.Sprintf("%.1fh", value)
		}),
		timeserieslinechart.WithAxesStyles(muted, muted),
	)
	for i, project := range projects {
		name := fmt.Sprintf("project-%d", dashboardProjectID(project))
		chart.SetDataSetStyle(
			name,
			lipgloss.NewStyle().Foreground(components.ChartColor(i)),
		)
		for _, bucket := range series[i] {
			chart.PushDataSet(name, timeserieslinechart.TimePoint{
				Time: bucket.Start, Value: bucket.Tracked.Hours(),
			})
		}
	}
	chart.DrawBrailleAll()
	return chart.View() + "\n" + legend
}

func (m Stats) visibleChartProjects() []storage.DashboardProjectSummary {
	result := make([]storage.DashboardProjectSummary, 0)
	for _, item := range m.list.VisibleItems() {
		project, ok := item.(statsProjectItem)
		if ok && project.project.Tracked > 0 {
			result = append(result, project.project)
		}
	}
	return result
}

func (m Stats) visibleEntries() []storage.Entry {
	projectIDs := make(map[int]bool)
	for _, item := range m.list.VisibleItems() {
		project, ok := item.(statsProjectItem)
		if ok {
			projectIDs[dashboardProjectID(project.project)] = true
		}
	}
	result := make([]storage.Entry, 0, len(m.entries))
	for _, entry := range m.entries {
		projectID := 0
		if entry.ProjectID != nil {
			projectID = *entry.ProjectID
		}
		if projectIDs[projectID] {
			result = append(result, entry)
		}
	}
	return result
}

func (m Stats) entriesForProject(projectID int) []storage.Entry {
	result := make([]storage.Entry, 0)
	for _, entry := range m.entries {
		entryProjectID := 0
		if entry.ProjectID != nil {
			entryProjectID = *entry.ProjectID
		}
		if entryProjectID == projectID {
			result = append(result, entry)
		}
	}
	return result
}

func chartLegend(
	projects []storage.DashboardProjectSummary,
	width int,
) string {
	lines := make([]string, 0, 1)
	line := ""
	for i, project := range projects {
		item := lipgloss.NewStyle().Foreground(components.ChartColor(i)).
			Render("●") + " " + project.ProjectName
		candidate := item
		if line != "" {
			candidate = line + "  " + item
		}
		if line != "" && lipgloss.Width(candidate) > width {
			lines = append(lines, line)
			line = item
		} else {
			line = candidate
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func timelineLabelFormatter(
	buckets []storage.DashboardTimeBucket,
) func(int, float64) string {
	location := buckets[0].Start.Location()
	layout := "Jan 02"
	step := buckets[1].Start.Sub(buckets[0].Start)
	if step <= 2*time.Hour {
		layout = "15:04"
	} else if step > 300*24*time.Hour {
		layout = "2006"
	} else if step > 20*24*time.Hour {
		layout = "Jan 06"
	}
	return func(_ int, value float64) string {
		return time.Unix(int64(value), 0).In(location).Format(layout)
	}
}

func (m *Stats) refresh() tea.Cmd {
	m.summary = storage.SummarizeDashboard(
		m.projects, m.tasks, m.rates, m.entries, m.payments,
		m.period.Start, m.period.End, m.now,
	)
	return m.refreshItems()
}

func (m *Stats) refreshItems() tea.Cmd {
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
	return m.refresh()
}

func (m Stats) openPeriodMenu() (Stats, tea.Cmd) {
	m.periodMenu = components.NewPeriodMenu(m.period, m.now, m.width)
	return m, m.periodMenu.Init()
}

func (m Stats) FormActive() bool { return m.periodMenu != nil }
func (m Stats) GlobalKeysEnabled() bool {
	return m.periodMenu == nil && m.list.FilterState() != list.Filtering
}
func (m Stats) Actions() string {
	return "[/] search  [f] filters  [F] reset filters  " +
		"[shift+up/down or K/J] period  " +
		"[shift+left/right or H/L] move  " +
		"[j/k] select  [enter] open tasks  [r] reload"
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

func formatStatsBalances(amounts map[string]int64) string {
	currencies := storage.SortedCurrencies(amounts)
	parts := make([]string, len(currencies))
	for i, currency := range currencies {
		parts[i] = components.FormatBalance(amounts[currency], currency)
	}
	return strings.Join(parts, ", ")
}
