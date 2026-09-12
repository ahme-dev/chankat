package screens

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"chankat/internal/storage"
	"chankat/internal/tui/components"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type Dashboard struct {
	ctx             context.Context
	stor            *storage.Storage
	entries         []storage.Entry
	projectList     []storage.Project
	taskList        []storage.Task
	projects        map[int]string
	tasks           map[int]string
	rates           map[int]storage.Rate
	active          list.Model
	activeItems     []storage.Entry
	taskPage        components.Page[taskItem]
	entryPage       *components.Page[entryItem]
	detailTask      *storage.Task
	focus           dashboardRow
	spinner         spinner.Model
	now             time.Time
	err             error
	loading         bool
	viewport        viewport.Model
	filter          *TaskListFilter
	filterMenu      *components.PeriodMenu
	filterProjectID *int
}

type TaskListFilter struct {
	ProjectID int
	Period    storage.Period
}

const allProjectsFilter = -1

type dashboardLoadedMsg struct {
	entries  []storage.Entry
	projects []storage.Project
	tasks    []storage.Task
	rates    []storage.Rate
}

type dashboardFailedMsg struct {
	err error
}

type dashboardTickMsg time.Time

type taskPausedMsg struct{}

type taskPauseFailedMsg struct {
	err error
}

type entryStartedMsg struct{}

type entryStartFailedMsg struct {
	err error
}

var (
	dashboardSectionStyle = list.DefaultStyles().StatusBar
	dashboardMutedStyle   = list.DefaultStyles().NoItems.PaddingLeft(2)
	dashboardErrorStyle   = lipgloss.NewStyle().Foreground(components.AccentColor)
)

type dashboardItem struct {
	title       string
	description string
}

func (i dashboardItem) Title() string       { return i.title }
func (i dashboardItem) Description() string { return i.description }
func (i dashboardItem) FilterValue() string { return i.title + " " + i.description }

func NewDashboard(ctx context.Context, stor *storage.Storage) Dashboard {
	filter := &TaskListFilter{
		ProjectID: allProjectsFilter,
		Period:    storage.Period{Kind: storage.All},
	}
	m := Dashboard{
		ctx:      ctx,
		stor:     stor,
		active:   newDashboardList(true),
		focus:    dashboardActiveRow,
		spinner:  spinner.New(),
		viewport: viewport.New(0, 0),
		now:      time.Now(),
		loading:  true,
		filter:   filter,
	}
	m.taskPage = newTaskPage(ctx, stor, filter, func() tea.Cmd {
		return loadDashboard(ctx, stor)
	})
	return m
}

func (m Dashboard) Init() tea.Cmd {
	return tea.Batch(
		loadDashboard(m.ctx, m.stor),
		m.taskPage.Init(),
		tickDashboard(),
		m.spinner.Tick,
	)
}

func (m Dashboard) Update(msg tea.Msg) (Dashboard, tea.Cmd) {
	if m.filterMenu != nil {
		if tick, ok := msg.(dashboardTickMsg); ok {
			periodChanged := m.updateFilterNow(time.Time(tick))
			m.refreshTables()
			var filterCmd tea.Cmd
			if periodChanged {
				filterCmd = m.refreshTaskPage()
			}
			return m, tea.Batch(tickDashboard(), filterCmd)
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
			m.closeFilterMenu()
			return m, nil
		}
		cmd := m.filterMenu.Update(msg)
		if m.filterMenu.Aborted() {
			m.closeFilterMenu()
			return m, nil
		}
		if m.filterMenu.Completed() {
			filterCmd := m.applyFilterDraft()
			m.closeFilterMenu()
			return m, tea.Batch(cmd, filterCmd)
		}
		return m, cmd
	}
	if _, ok := msg.(dashboardTickMsg); ok {
		periodChanged := m.updateFilterNow(time.Time(msg.(dashboardTickMsg)))
		m.refreshTables()
		var filterCmd tea.Cmd
		if periodChanged {
			filterCmd = m.refreshTaskPage()
		}
		return m, tea.Batch(tickDashboard(), filterCmd)
	}
	if m.entryPage != nil && m.entryPage.FormActive() {
		var cmd tea.Cmd
		*m.entryPage, cmd = m.entryPage.Update(msg)
		return m, cmd
	}
	if m.taskPage.FormActive() {
		var cmd tea.Cmd
		m.taskPage, cmd = m.taskPage.Update(msg)
		return m, cmd
	}

	var taskCmd, entryCmd tea.Cmd
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg, tea.WindowSizeMsg:
	default:
		m.taskPage, taskCmd = m.taskPage.Update(msg)
		if m.entryPage != nil {
			*m.entryPage, entryCmd = m.entryPage.Update(msg)
		}
		m.resizeTaskPage()
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height
		m.active.SetSize(msg.Width, m.active.Height())
		m.resizeTaskPage()
		m.resizeEntryPage()
		return m, nil
	case dashboardLoadedMsg:
		m.now = time.Now()
		m.entries = msg.entries
		m.projectList = msg.projects
		m.taskList = msg.tasks
		m.projects = projectNames(msg.projects)
		m.tasks = taskNames(msg.tasks)
		m.rates = ratesByID(msg.rates)
		detailChanged := m.refreshDetailTask()
		if detailChanged && m.entryPage != nil {
			entryCmd = m.entryPage.Reload()
		}
		m.loading = false
		m.err = nil
		m.refreshTables()
		taskCmd = tea.Batch(taskCmd, m.refreshTaskPage())
	case dashboardFailedMsg:
		m.loading = false
		m.err = msg.err
	case taskPausedMsg:
		m.loading = true
		return m, tea.Batch(
			loadDashboard(m.ctx, m.stor),
			m.taskPage.Reload(),
		)
	case entryStartedMsg:
		m.loading = true
		m.taskPage.ResetFilter()
		return m, tea.Batch(
			loadDashboard(m.ctx, m.stor),
			m.taskPage.Reload(),
		)
	case taskPauseFailedMsg:
		m.err = msg.err
	case entryStartFailedMsg:
		m.err = msg.err
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case tea.KeyMsg:
		if m.detailTask != nil {
			return m.updateDetail(msg, tea.Batch(taskCmd, entryCmd))
		}
		if !m.taskPage.GlobalKeysEnabled() {
			m.taskPage, taskCmd = m.taskPage.Update(msg)
			m.resizeTaskPage()
			return m, taskCmd
		}
		switch msg.String() {
		case "f":
			return m.openFilterMenu()
		case "F":
			return m, m.resetFilter()
		case "shift+left", "H":
			m.filter.Period = storage.MovePeriod(m.filter.Period, -1)
			m.refreshTables()
			return m, m.refreshTaskPage()
		case "shift+right", "L":
			return m, m.moveFilterForward()
		case "shift+up", "K":
			m.filter.Period = storage.StepPeriodKind(m.filter.Period, -1, m.now)
			m.refreshTables()
			return m, m.refreshTaskPage()
		case "shift+down", "J":
			m.filter.Period = storage.StepPeriodKind(m.filter.Period, 1, m.now)
			m.refreshTables()
			return m, m.refreshTaskPage()
		case "/", "n":
			m.setFocus(dashboardTaskRow)
			m.taskPage, taskCmd = m.taskPage.Update(msg)
			return m, taskCmd
		case "a":
			return m.openHistoricalTask()
		case "enter":
			return m.openSelectedTask()
		case "e":
			return m.openSelectedTaskForm(false)
		case "x", "delete":
			return m.openSelectedTaskForm(true)
		case "c":
			if m.focus == dashboardTaskRow {
				m.taskPage, taskCmd = m.taskPage.Update(msg)
				return m, taskCmd
			}
			return m, nil
		case " ", "space":
			switch m.focus {
			case dashboardActiveRow:
				cursor := m.active.Index()
				if cursor >= 0 && cursor < len(m.activeItems) {
					return m, pauseTask(
						m.ctx, m.stor, m.activeItems[cursor], m.now,
					)
				}
			case dashboardTaskRow:
				if item, ok := m.taskPage.Selected(); ok {
					return m, startTaskEntry(
						m.ctx, m.stor, item.task, m.now,
					)
				}
			}
			return m, nil
		case "down", "j":
			if m.focus == dashboardActiveRow &&
				m.active.Index() == len(m.activeItems)-1 &&
				m.taskPage.VisibleCount() > 0 {
				m.setFocus(dashboardTaskRow)
				m.taskPage.Select(0)
				return m, nil
			}
		case "up", "k":
			if m.focus == dashboardTaskRow &&
				m.taskPage.Index() <= 0 &&
				len(m.activeItems) > 0 {
				m.setFocus(dashboardActiveRow)
				m.active.Select(len(m.activeItems) - 1)
				return m, nil
			}
		}
	case tea.MouseMsg:
		if m.detailTask != nil {
			if m.entryPage != nil {
				*m.entryPage, entryCmd = m.entryPage.Update(msg)
			}
			return m, entryCmd
		}
		mouse := tea.MouseEvent(msg)
		if mouse.Button == tea.MouseButtonWheelUp ||
			mouse.Button == tea.MouseButtonWheelDown {
			var cmd tea.Cmd
			m.viewport.SetContent(m.content())
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		if mouse.Button != tea.MouseButtonLeft ||
			mouse.Action != tea.MouseActionPress {
			break
		}
		kind, index := m.rowAt(mouse.Y - 2 + m.viewport.YOffset)
		switch kind {
		case dashboardActiveRow:
			m.setFocus(dashboardActiveRow)
			m.active.Select(index)
			if dashboardActionAt(kind, mouse.X) {
				return m, pauseTask(
					m.ctx, m.stor, m.activeItems[index], m.now,
				)
			}
			return m.openSelectedTask()
		case dashboardTaskRow:
			m.setFocus(dashboardTaskRow)
			m.taskPage.Select(index)
			if dashboardActionAt(kind, mouse.X) {
				item, ok := m.taskPage.Selected()
				if !ok {
					return m, nil
				}
				return m, startTaskEntry(
					m.ctx, m.stor, item.task, m.now,
				)
			}
			return m.openSelectedTask()
		}
	}

	if !m.loading {
		var cmd tea.Cmd
		if m.focus == dashboardTaskRow {
			m.taskPage, cmd = m.taskPage.Update(msg)
		} else {
			m.active, cmd = m.active.Update(msg)
		}
		m.ensureFocusVisible()
		return m, tea.Batch(cmd, taskCmd, entryCmd)
	}
	return m, tea.Batch(taskCmd, entryCmd)
}

func (m Dashboard) View() string {
	if m.filterMenu != nil {
		return "tasks / filters\n\n" + m.filterMenu.View() + "\n\n[esc] back"
	}
	if m.entryPage != nil && m.entryPage.FormActive() {
		return m.entryPage.View()
	}
	if m.taskPage.FormActive() {
		return m.taskPage.View()
	}
	if m.loading {
		return m.spinner.View() + " Loading..."
	}
	if m.err != nil {
		return dashboardErrorStyle.Render("Error: " + m.err.Error())
	}
	if m.detailTask != nil {
		return m.detailView()
	}

	content := m.content()
	if m.viewport.Height < 1 {
		return content
	}
	view := m.viewport
	view.SetContent(content)
	return view.View()
}

func (m Dashboard) content() string {
	var sections []string
	sections = append(sections, dashboardMutedStyle.Render(m.filterLabel()))
	var active strings.Builder
	active.WriteString(dashboardSectionStyle.Render(
		countLabel(len(m.activeItems), "active task"),
	) + "\n")
	if len(m.active.Items()) == 0 {
		active.WriteString(dashboardMutedStyle.Render("No active timers."))
	} else {
		active.WriteString(m.active.View())
	}
	sections = append(sections, active.String())

	var available strings.Builder
	available.WriteString(dashboardSectionStyle.Render(
		countLabel(m.taskPage.VisibleCount(), "task"),
	))
	if m.taskPage.VisibleCount() == 0 {
		available.WriteString(dashboardMutedStyle.Render("No inactive tasks."))
	} else {
		available.WriteString(m.taskPage.View())
	}
	sections = append(sections, available.String())

	return strings.Join(sections, "\n\n")
}

func countLabel(count int, singular string) string {
	label := singular
	if count != 1 {
		label += "s"
	}
	return fmt.Sprintf("%d %s", count, label)
}

func (m *Dashboard) refreshTables() {
	entries := m.filteredEntries()
	active := make([]list.Item, 0)
	m.activeItems = m.activeItems[:0]
	for _, entry := range entries {
		if entry.EndedAt == nil {
			m.activeItems = append(m.activeItems, entry)
			duration := m.now.Sub(entry.StartedAt)
			if duration < 0 {
				duration = 0
			}
			amounts := make(map[string]int64)
			if entry.TaskID != nil {
				duration, amounts = taskTotals(
					entries,
					m.rates,
					*entry.TaskID,
					m.now,
				)
			} else if entry.RateID != nil {
				if rate, ok := m.rates[*entry.RateID]; ok {
					seconds := int64(duration / time.Second)
					amounts[rate.Currency] =
						int64(rate.AmountMinor) * seconds / 3600
				}
			}
			session := m.now.Sub(entry.StartedAt)
			if session < 0 {
				session = 0
			}
			description := []string{
				m.entryProject(entry),
			}
			if entry.RateID != nil {
				if rate, ok := m.rates[*entry.RateID]; ok {
					description = append(description, fmt.Sprintf(
						"%s · %s/hour",
						rate.Name,
						components.FormatMoney(
							int64(rate.AmountMinor), rate.Currency,
						),
					))
				}
			}
			description = append(
				description,
				"session "+components.FormatDuration(session),
				"total "+components.FormatDuration(duration),
			)
			if amount := formatTaskAmounts(amounts); amount != "" {
				description = append(description, amount+" earned")
			}
			active = append(active, dashboardItem{
				title:       "[||] " + m.entryTask(entry),
				description: strings.Join(description, " · "),
			})
		}
	}

	setDashboardItems(&m.active, active, true)
	if m.focus == dashboardActiveRow && len(m.activeItems) == 0 {
		m.focus = dashboardTaskRow
	}
	if m.focus == dashboardTaskRow && m.taskPage.VisibleCount() == 0 {
		m.focus = dashboardActiveRow
	}
	m.setFocus(m.focus)
}

func (m Dashboard) filteredEntries() []storage.Entry {
	entries := m.entries
	if m.filter == nil {
		return entries
	}
	periodFiltered := m.filter.Period.Kind != storage.All ||
		!m.filter.Period.Start.IsZero() || !m.filter.Period.End.IsZero()
	if periodFiltered {
		entries = storage.EntriesInPeriod(entries, m.filter.Period, m.now)
	}
	if m.filter.ProjectID != allProjectsFilter {
		entries = entriesForProject(entries, m.filter.ProjectID)
	}
	return entries
}

func entriesForProject(entries []storage.Entry, projectID int) []storage.Entry {
	result := make([]storage.Entry, 0, len(entries))
	for _, entry := range entries {
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

func (m *Dashboard) refreshTaskPage() tea.Cmd {
	if m.filter == nil {
		return nil
	}
	return m.taskPage.SetItems(taskItemsForFilter(
		m.taskList,
		m.projectList,
		m.entries,
		mapRates(m.rates),
		*m.filter,
		m.now,
	))
}

func mapRates(rates map[int]storage.Rate) []storage.Rate {
	result := make([]storage.Rate, 0, len(rates))
	for _, rate := range rates {
		result = append(result, rate)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (m *Dashboard) ApplyFilter(projectID int, period storage.Period) tea.Cmd {
	if m.filter == nil {
		m.filter = &TaskListFilter{}
	}
	m.filter.ProjectID = projectID
	m.filter.Period = period
	m.detailTask = nil
	m.entryPage = nil
	m.taskPage.ResetFilter()
	m.viewport.SetYOffset(0)
	m.refreshTables()
	return tea.Batch(m.refreshTaskPage(), loadDashboard(m.ctx, m.stor))
}

func (m *Dashboard) resetFilter() tea.Cmd {
	return m.ApplyFilter(
		allProjectsFilter,
		storage.Period{Kind: storage.All},
	)
}

func (m Dashboard) openFilterMenu() (Dashboard, tea.Cmd) {
	projectID := allProjectsFilter
	if m.filter != nil {
		projectID = m.filter.ProjectID
	}
	m.filterProjectID = &projectID
	projectOptions := []huh.Option[int]{
		huh.NewOption("All projects", allProjectsFilter),
		huh.NewOption("No project", 0),
	}
	for _, project := range m.projectList {
		projectOptions = append(
			projectOptions, huh.NewOption(project.Name, project.ID),
		)
	}
	projectField := huh.NewSelect[int]().
		Title("Project").
		Options(projectOptions...).
		Value(m.filterProjectID)
	period := storage.Period{Kind: storage.All}
	if m.filter != nil {
		period = m.filter.Period
	}
	m.filterMenu = components.NewPeriodMenu(
		period,
		m.now,
		m.viewport.Width,
		projectField,
	)
	return m, m.filterMenu.Init()
}

func (m *Dashboard) applyFilterDraft() tea.Cmd {
	if m.filterMenu == nil || m.filterProjectID == nil {
		return nil
	}
	period, err := m.filterMenu.Period(m.now)
	if err != nil {
		m.err = err
		return nil
	}
	m.err = nil
	return m.ApplyFilter(*m.filterProjectID, period)
}

func (m *Dashboard) closeFilterMenu() {
	m.filterMenu = nil
	m.filterProjectID = nil
}

func (m *Dashboard) moveFilterForward() tea.Cmd {
	if m.filter == nil || m.filter.Period.Kind == storage.All ||
		m.filter.Period.Kind == "" {
		return nil
	}
	current, err := storage.CurrentPeriod(m.filter.Period.Kind, m.now)
	if err != nil || !m.filter.Period.Start.Before(current.Start) {
		return nil
	}
	m.filter.Period = storage.MovePeriod(m.filter.Period, 1)
	m.refreshTables()
	return m.refreshTaskPage()
}

func (m *Dashboard) updateFilterNow(now time.Time) bool {
	if m.filter == nil {
		m.now = now
		return false
	}
	previous := m.filter.Period
	followsCurrent := false
	if previous.Kind != "" && previous.Kind != storage.All {
		current, err := storage.CurrentPeriod(previous.Kind, m.now)
		followsCurrent = err == nil && previous.Start.Equal(current.Start) &&
			previous.End.Equal(current.End)
	}
	m.now = now
	if followsCurrent {
		current, err := storage.CurrentPeriod(previous.Kind, now)
		if err == nil {
			m.filter.Period = current
		}
	}
	return !previous.Start.Equal(m.filter.Period.Start) ||
		!previous.End.Equal(m.filter.Period.End)
}

func (m Dashboard) filterLabel() string {
	project := "All projects"
	period := storage.Period{Kind: storage.All}
	if m.filter != nil {
		period = m.filter.Period
		switch m.filter.ProjectID {
		case allProjectsFilter:
		case 0:
			project = "No project"
		default:
			project = m.projects[m.filter.ProjectID]
			if project == "" {
				project = fmt.Sprintf("Project %d", m.filter.ProjectID)
			}
		}
	}
	return "Filter: " + project + " · " + period.Label()
}

func newDashboardList(focused bool) list.Model {
	delegate := components.NewInactiveListDelegate()
	if focused {
		delegate = components.NewListDelegate()
	}
	model := list.New(nil, delegate, 80, 1)
	model.SetShowTitle(false)
	model.SetShowStatusBar(false)
	model.SetFilteringEnabled(false)
	model.SetShowPagination(false)
	model.SetShowHelp(false)
	return model
}

func setDashboardItems(target *list.Model, items []list.Item, focused bool) {
	cursor := target.Index()
	target.SetItems(items)
	if focused {
		target.SetDelegate(components.NewListDelegate())
	} else {
		target.SetDelegate(components.NewInactiveListDelegate())
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(items) {
		cursor = len(items) - 1
	}
	if len(items) > 0 {
		target.Select(cursor)
	}
	height := len(items) * 2
	if height < 1 {
		height = 1
	}
	target.SetSize(target.Width(), height)
}

func (m *Dashboard) setFocus(focus dashboardRow) {
	m.focus = focus
	m.active.SetDelegate(components.NewInactiveListDelegate())
	m.taskPage.SetFocused(false)
	if focus == dashboardTaskRow {
		m.taskPage.SetFocused(true)
	} else {
		m.active.SetDelegate(components.NewListDelegate())
	}
}

func (m *Dashboard) ensureFocusVisible() {
	const filterHeight = 2
	var y int
	switch m.focus {
	case dashboardActiveRow:
		if len(m.activeItems) == 0 {
			return
		}
		y = filterHeight + 2 + m.active.Index()*2
	case dashboardTaskRow:
		if m.taskPage.VisibleCount() == 0 {
			return
		}
		activeBodyHeight := 1
		if len(m.activeItems) > 0 {
			activeBodyHeight = len(m.activeItems) * 2
		}
		y = filterHeight + 3 + activeBodyHeight + 2 + m.taskPage.Index()*2
	default:
		return
	}

	if y < m.viewport.YOffset {
		m.viewport.SetYOffset(y)
	} else if y >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(y - m.viewport.Height + 1)
	}
}

type dashboardRow int

const (
	dashboardNoRow dashboardRow = iota
	dashboardActiveRow
	dashboardTaskRow
)

func (m Dashboard) rowAt(y int) (dashboardRow, int) {
	const filterHeight = 2
	activeBodyHeight := 1
	if len(m.activeItems) > 0 {
		activeBodyHeight = len(m.activeItems) * 2
		if offset := y - filterHeight - 2; offset >= 0 && offset%2 == 0 {
			if index := offset / 2; index < len(m.activeItems) {
				return dashboardActiveRow, index
			}
		}
	}

	taskTitle := filterHeight + 3 + activeBodyHeight
	if m.taskPage.VisibleCount() > 0 {
		if offset := y - taskTitle - 2; offset >= 0 && offset%2 == 0 {
			if index := offset / 2; index < m.taskPage.VisibleCount() {
				return dashboardTaskRow, index
			}
		}
	}
	return dashboardNoRow, -1
}

func dashboardActionAt(row dashboardRow, x int) bool {
	switch row {
	case dashboardActiveRow:
		return x >= 2 && x < 2+len("[||]")
	case dashboardTaskRow:
		return x >= 2 && x < 2+len("[>]")
	default:
		return false
	}
}

func pauseTask(
	ctx context.Context,
	stor *storage.Storage,
	entry storage.Entry,
	endedAt time.Time,
) tea.Cmd {
	return func() tea.Msg {
		if entry.TaskID == nil {
			return taskPauseFailedMsg{
				err: fmt.Errorf("active entry %d has no task", entry.ID),
			}
		}
		if err := stor.PauseTask(ctx, *entry.TaskID, endedAt); err != nil {
			return taskPauseFailedMsg{err: err}
		}
		return taskPausedMsg{}
	}
}

func startTaskEntry(
	ctx context.Context,
	stor *storage.Storage,
	task storage.Task,
	startedAt time.Time,
) tea.Cmd {
	return func() tea.Msg {
		if err := stor.StartTask(ctx, task.ID, startedAt); err != nil {
			return entryStartFailedMsg{err: err}
		}
		return entryStartedMsg{}
	}
}

func loadDashboard(ctx context.Context, stor *storage.Storage) tea.Cmd {
	return func() tea.Msg {
		entries, err := stor.GetEntries(ctx)
		if err != nil {
			return dashboardFailedMsg{err: err}
		}
		projects, err := stor.GetProjects(ctx)
		if err != nil {
			return dashboardFailedMsg{err: err}
		}
		tasks, err := stor.GetTasks(ctx)
		if err != nil {
			return dashboardFailedMsg{err: err}
		}
		rates, err := stor.GetRates(ctx)
		if err != nil {
			return dashboardFailedMsg{err: err}
		}
		return dashboardLoadedMsg{
			entries: entries, projects: projects, tasks: tasks, rates: rates,
		}
	}
}

func tickDashboard() tea.Cmd {
	return tea.Tick(time.Second, func(now time.Time) tea.Msg {
		return dashboardTickMsg(now)
	})
}

func (m Dashboard) entryProject(entry storage.Entry) string {
	project := "No project"
	if entry.ProjectID != nil {
		project = m.projects[*entry.ProjectID]
	}
	return project
}

func (m Dashboard) entryTask(entry storage.Entry) string {
	task := "No task"
	if entry.TaskID != nil {
		task = m.tasks[*entry.TaskID]
	}
	return task
}

func (m Dashboard) entryAmount(entry storage.Entry, endedAt time.Time) string {
	if entry.RateID == nil {
		return "-"
	}
	rate, ok := m.rates[*entry.RateID]
	if !ok {
		return "-"
	}
	seconds := int64(endedAt.Sub(entry.StartedAt) / time.Second)
	if seconds < 0 {
		seconds = 0
	}
	amount := int64(rate.AmountMinor) * seconds / 3600
	return components.FormatMoney(amount, rate.Currency)
}

func projectNames(projects []storage.Project) map[int]string {
	names := make(map[int]string, len(projects))
	for _, project := range projects {
		names[project.ID] = project.Name
	}
	return names
}

func taskNames(tasks []storage.Task) map[int]string {
	names := make(map[int]string, len(tasks))
	for _, task := range tasks {
		names[task.ID] = task.Name
	}
	return names
}

func ratesByID(rates []storage.Rate) map[int]storage.Rate {
	byID := make(map[int]storage.Rate, len(rates))
	for _, rate := range rates {
		byID[rate.ID] = rate
	}
	return byID
}

func (m Dashboard) FormActive() bool {
	return m.filterMenu != nil || m.taskPage.FormActive() ||
		m.entryPage != nil && m.entryPage.FormActive()
}

func (m Dashboard) GlobalKeysEnabled() bool {
	if m.filterMenu != nil {
		return false
	}
	if m.entryPage != nil && m.entryPage.FormActive() {
		return false
	}
	if m.detailTask != nil && m.entryPage != nil {
		return m.entryPage.GlobalKeysEnabled()
	}
	return m.taskPage.GlobalKeysEnabled()
}

func (m *Dashboard) Reload() tea.Cmd {
	m.loading = true
	var entryCmd tea.Cmd
	if m.entryPage != nil {
		entryCmd = m.entryPage.Reload()
	}
	return tea.Batch(loadDashboard(m.ctx, m.stor), m.taskPage.Reload(), entryCmd)
}

func (m Dashboard) Actions() string {
	if m.detailTask != nil {
		return "[/] search  [n] add time  [e/enter] edit time  " +
			"[x/delete] delete time  [t] edit task  [esc] back"
	}
	return "[/] search  [n] new & track  [a] add past task  " +
		"[enter] details  [e] edit task  [x/delete] archive  " +
		"[space] start/pause  [f] filters  [F] reset filters  " +
		"[shift+up/down or K/J] period  " +
		"[shift+left/right or H/L] move"
}

func (m *Dashboard) resizeTaskPage() {
	activeBodyHeight := len(m.activeItems) * 2
	if activeBodyHeight < 1 {
		activeBodyHeight = 1
	}
	height := m.viewport.Height - activeBodyHeight - 8
	if height < 4 {
		height = 4
	}
	m.taskPage.SetSize(m.viewport.Width, height)
}

func (m *Dashboard) resizeEntryPage() {
	if m.entryPage == nil {
		return
	}
	height := m.viewport.Height - 3
	if height < 4 {
		height = 4
	}
	m.entryPage.SetSize(m.viewport.Width, height)
}

func (m Dashboard) updateDetail(
	msg tea.KeyMsg,
	pending tea.Cmd,
) (Dashboard, tea.Cmd) {
	if m.entryPage == nil {
		m.detailTask = nil
		return m, pending
	}
	if !m.entryPage.GlobalKeysEnabled() {
		var cmd tea.Cmd
		*m.entryPage, cmd = m.entryPage.Update(msg)
		return m, tea.Batch(pending, cmd)
	}
	switch msg.String() {
	case "esc":
		m.detailTask = nil
		m.entryPage = nil
		return m, pending
	case "t":
		item, ok := m.taskItem(*m.detailTask)
		if !ok {
			return m, nil
		}
		form, err := taskForm(
			m.ctx, m.stor, &item.task, m.projectList, mapRates(m.rates),
		)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.taskPage, pending = m.taskPage.OpenForm(form)
		return m, pending
	default:
		var cmd tea.Cmd
		*m.entryPage, cmd = m.entryPage.Update(msg)
		return m, tea.Batch(pending, cmd)
	}
}

func (m Dashboard) openSelectedTask() (Dashboard, tea.Cmd) {
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	page := newEntryPage(m.ctx, m.stor, task.ID, m.filter, func() tea.Cmd {
		return tea.Batch(
			loadDashboard(m.ctx, m.stor),
			m.taskPage.Reload(),
		)
	})
	m.detailTask = &task
	m.entryPage = &page
	m.viewport.SetYOffset(0)
	m.resizeEntryPage()
	return m, m.entryPage.Init()
}

func (m Dashboard) openSelectedTaskForm(deleteTask bool) (Dashboard, tea.Cmd) {
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	var form *components.Form[taskItem]
	if deleteTask {
		form = archiveTaskForm(m.ctx, m.stor, task)
	} else {
		var err error
		form, err = taskForm(
			m.ctx, m.stor, &task, m.projectList, mapRates(m.rates),
		)
		if err != nil {
			m.err = err
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.taskPage, cmd = m.taskPage.OpenForm(form)
	return m, cmd
}

func (m Dashboard) openHistoricalTask() (Dashboard, tea.Cmd) {
	form, err := historicalTaskForm(
		m.ctx, m.stor, m.projectList, mapRates(m.rates), m.now,
	)
	if err != nil {
		m.err = err
		return m, nil
	}
	var cmd tea.Cmd
	m.taskPage, cmd = m.taskPage.OpenForm(form)
	return m, cmd
}

func (m Dashboard) selectedTask() (storage.Task, bool) {
	if m.focus == dashboardTaskRow {
		if item, ok := m.taskPage.Selected(); ok {
			return item.task, true
		}
		return storage.Task{}, false
	}
	index := m.active.Index()
	if index < 0 || index >= len(m.activeItems) {
		return storage.Task{}, false
	}
	entry := m.activeItems[index]
	if entry.TaskID == nil {
		return storage.Task{}, false
	}
	for _, task := range m.taskList {
		if task.ID == *entry.TaskID {
			return task, true
		}
	}
	return storage.Task{}, false
}

func (m Dashboard) taskItem(task storage.Task) (taskItem, bool) {
	for _, project := range m.projectList {
		if project.ID == task.ProjectID {
			return taskItem{task: task, project: project}, true
		}
	}
	return taskItem{}, false
}

func (m *Dashboard) refreshDetailTask() bool {
	if m.detailTask == nil {
		return false
	}
	previous := *m.detailTask
	id := m.detailTask.ID
	for _, task := range m.taskList {
		if task.ID == id {
			copy := task
			m.detailTask = &copy
			return !sameTask(task, previous)
		}
	}
	m.detailTask = nil
	m.entryPage = nil
	return true
}

func sameTask(left, right storage.Task) bool {
	if left.ID != right.ID || left.Name != right.Name ||
		left.ProjectID != right.ProjectID {
		return false
	}
	if left.RateID == nil || right.RateID == nil {
		return left.RateID == nil && right.RateID == nil
	}
	return *left.RateID == *right.RateID
}

func (m Dashboard) detailView() string {
	task := *m.detailTask
	project := m.projects[task.ProjectID]
	duration, amounts := taskTotals(m.filteredEntries(), m.rates, task.ID, m.now)
	rate, overridden := effectiveTaskRate(task, m.projectList, m.rates)
	summary := project
	for _, item := range storage.SummarizeTasks([]storage.Task{task}, m.projectList,
		m.filteredEntries(), mapRates(m.rates), m.now) {
		summary += formatHistoricalRates(item.HistoricalRates)
	}
	if rate.ID != 0 {
		summary += " · " + formatTaskRate(rate, overridden)
	}
	summary += " · total " + components.FormatDuration(duration)
	if amount := formatTaskAmounts(amounts); amount != "" {
		summary += " · " + amount + " earned"
	}
	body := "No entries."
	if m.entryPage != nil {
		body = m.entryPage.View()
	}
	return fmt.Sprintf(
		"%s\n%s\n\n%s\n%s",
		dashboardSectionStyle.Render(task.Name),
		dashboardMutedStyle.Render(summary),
		dashboardSectionStyle.Render("Time entries"),
		body,
	)
}

type taskItem struct {
	task        storage.Task
	project     storage.Project
	rate        storage.Rate
	description string
}

func (t taskItem) Title() string       { return "[>] " + t.task.Name }
func (t taskItem) Description() string { return t.description }
func (t taskItem) FilterValue() string {
	return t.task.Name + " " + t.project.Name + " " + t.rate.Name
}

type taskFormValues struct {
	name      string
	projectID int
	rateID    int
}

type taskFormMeta struct {
	projects []storage.Project
	rates    []storage.Rate
}

func newTaskPage(
	ctx context.Context,
	stor *storage.Storage,
	filter *TaskListFilter,
	afterSave func() tea.Cmd,
) components.Page[taskItem] {
	config := components.Config[taskItem]{
		Name:     "tasks",
		Embedded: true,
		Load: func(ctx context.Context) ([]taskItem, any, error) {
			tasks, err := stor.GetTasks(ctx)
			if err != nil {
				return nil, nil, err
			}
			projects, err := stor.GetProjects(ctx)
			if err != nil {
				return nil, nil, err
			}
			entries, err := stor.GetEntries(ctx)
			if err != nil {
				return nil, nil, err
			}
			rates, err := stor.GetRates(ctx)
			if err != nil {
				return nil, nil, err
			}
			return taskItemsForFilter(
				tasks, projects, entries, rates, *filter, time.Now(),
			), taskFormMeta{projects: projects, rates: rates}, nil
		},
		Create: func(meta any) (*components.Form[taskItem], error) {
			values := meta.(taskFormMeta)
			return taskForm(ctx, stor, nil, values.projects, values.rates)
		},
		Update: func(item taskItem, meta any) (*components.Form[taskItem], error) {
			values := meta.(taskFormMeta)
			return taskForm(ctx, stor, &item.task, values.projects, values.rates)
		},
		Delete: func(item taskItem) *components.Form[taskItem] {
			return archiveTaskForm(ctx, stor, item.task)
		},
		AfterSave: afterSave,
	}
	return components.NewPage(ctx, config)
}

func taskItems(
	tasks []storage.Task,
	projects []storage.Project,
	entries []storage.Entry,
	rates []storage.Rate,
) []taskItem {
	return summarizedTaskItems(
		tasks, projects, entries, rates, time.Time{}, false,
	)
}

func taskItemsForFilter(
	tasks []storage.Task,
	projects []storage.Project,
	entries []storage.Entry,
	rates []storage.Rate,
	filter TaskListFilter,
	now time.Time,
) []taskItem {
	periodFiltered := filter.Period.Kind != storage.All ||
		!filter.Period.Start.IsZero() || !filter.Period.End.IsZero()
	if periodFiltered {
		entries = storage.EntriesInPeriod(entries, filter.Period, now)
	}
	if filter.ProjectID != allProjectsFilter {
		filteredTasks := make([]storage.Task, 0, len(tasks))
		for _, task := range tasks {
			if task.ProjectID == filter.ProjectID {
				filteredTasks = append(filteredTasks, task)
			}
		}
		tasks = filteredTasks
		entries = entriesForProject(entries, filter.ProjectID)
	}
	if periodFiltered {
		taskIDs := make(map[int]bool)
		for _, entry := range entries {
			if entry.TaskID != nil {
				taskIDs[*entry.TaskID] = true
			}
		}
		filteredTasks := make([]storage.Task, 0, len(tasks))
		for _, task := range tasks {
			if taskIDs[task.ID] {
				filteredTasks = append(filteredTasks, task)
			}
		}
		tasks = filteredTasks
	}
	totalsNow := time.Time{}
	if periodFiltered {
		totalsNow = now
	}
	return summarizedTaskItems(
		tasks, projects, entries, rates, totalsNow, periodFiltered,
	)
}

func summarizedTaskItems(
	tasks []storage.Task,
	projects []storage.Project,
	entries []storage.Entry,
	rates []storage.Rate,
	now time.Time,
	periodFiltered bool,
) []taskItem {
	summaries := storage.SummarizeTasks(
		tasks, projects, entries, rates, now,
	)
	ordered := make([]storage.TaskSummary, 0, len(summaries))
	for _, summary := range summaries {
		if !summary.Active {
			ordered = append(ordered, summary)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		leftOK := left.LastEndedAt != nil
		rightOK := right.LastEndedAt != nil
		if leftOK != rightOK {
			return leftOK
		}
		if !leftOK {
			return false
		}
		if !left.LastEndedAt.Equal(*right.LastEndedAt) {
			return left.LastEndedAt.After(*right.LastEndedAt)
		}
		return left.LastEntryID > right.LastEntryID
	})

	items := make([]taskItem, 0, len(ordered))
	for _, summary := range ordered {
		description := summary.Project.Name
		description += formatHistoricalRates(summary.HistoricalRates)
		if summary.Rate.ID != 0 {
			description += " · " + formatTaskRate(
				summary.Rate, summary.RateOverridden,
			)
		}
		if summary.LastEndedAt != nil {
			totalLabel := "total "
			if periodFiltered {
				totalLabel = "period "
			}
			description += " · " + totalLabel +
				components.FormatDuration(summary.Tracked)
			if amount := formatTaskAmounts(summary.EarnedMinor); amount != "" {
				description += " · " + amount + " earned"
			}
			description += " · worked " +
				components.FormatDate(*summary.LastEndedAt)
		}
		items = append(items, taskItem{
			task:        summary.Task,
			project:     summary.Project,
			rate:        summary.Rate,
			description: description,
		})
	}
	return items
}

func taskTotals(
	entries []storage.Entry,
	rates map[int]storage.Rate,
	taskID int,
	now time.Time,
) (time.Duration, map[string]int64) {
	return storage.TaskTotals(entries, rates, taskID, now)
}

func formatTaskAmounts(amounts map[string]int64) string {
	if len(amounts) == 0 {
		return ""
	}
	currencies := make([]string, 0, len(amounts))
	for currency := range amounts {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	formatted := make([]string, len(currencies))
	for i, currency := range currencies {
		formatted[i] = components.FormatMoney(amounts[currency], currency)
	}
	return strings.Join(formatted, ", ")
}

func formatTaskRate(rate storage.Rate, overridden bool) string {
	source := "project rate"
	if overridden {
		source = "task rate"
	}
	return fmt.Sprintf(
		"Next rate: %s · %s/hour (%s)",
		rate.Name,
		components.FormatMoney(int64(rate.AmountMinor), rate.Currency),
		source,
	)
}

func formatHistoricalRates(rates []storage.Rate) string {
	if len(rates) == 0 {
		return ""
	}
	parts := make([]string, len(rates))
	for i, rate := range rates {
		parts[i] = fmt.Sprintf(
			"%s %s/hour", rate.Name,
			components.FormatMoney(int64(rate.AmountMinor), rate.Currency),
		)
	}
	return " · Used: " + strings.Join(parts, ", ")
}

func effectiveTaskRate(
	task storage.Task,
	projects []storage.Project,
	rates map[int]storage.Rate,
) (storage.Rate, bool) {
	if task.RateID != nil {
		return rates[*task.RateID], true
	}
	for _, project := range projects {
		if project.ID == task.ProjectID {
			return rates[project.RateID], false
		}
	}
	return storage.Rate{}, false
}

func taskForm(
	ctx context.Context,
	stor *storage.Storage,
	task *storage.Task,
	projects []storage.Project,
	rates []storage.Rate,
) (*components.Form[taskItem], error) {
	form, values, err := taskFields(task, projects, rates)
	if err != nil {
		return nil, err
	}
	action := "new and track"
	if task != nil {
		action = "edit"
	}
	return components.NewForm[taskItem](
		ctx,
		"tasks / "+action,
		form,
		func(ctx context.Context) error {
			value := storage.Task{
				Name: strings.TrimSpace(values.name), ProjectID: values.projectID,
				RateID: optionalTaskRateID(values.rateID),
			}
			if task == nil {
				return stor.CreateTaskAndStart(ctx, value, time.Now())
			}
			value.ID = task.ID
			return stor.UpdateTask(ctx, value)
		},
	), nil
}

func archiveTaskForm(
	ctx context.Context,
	stor *storage.Storage,
	task storage.Task,
) *components.Form[taskItem] {
	confirmed := false
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Archive " + task.Name + "?").
			Description("Stops running timers and preserves time entries and earnings.").
			Affirmative("Archive").
			Negative("Cancel").
			Value(&confirmed),
	))
	return components.NewForm[taskItem](
		ctx,
		"archive task",
		form,
		func(ctx context.Context) error {
			if !confirmed {
				return nil
			}
			return stor.DeleteTask(ctx, task.ID)
		},
	)
}

func historicalTaskForm(
	ctx context.Context,
	stor *storage.Storage,
	projects []storage.Project,
	rates []storage.Rate,
	now time.Time,
) (*components.Form[taskItem], error) {
	if len(projects) == 0 {
		return nil, errors.New("no projects available; create a project first")
	}
	now = now.Truncate(time.Minute)
	name := ""
	projectID := projects[0].ID
	rateID := 0
	startedAt := components.FormatDateTime(now.Add(-time.Hour))
	endedAt := components.FormatDateTime(now)
	note := ""
	options := make([]huh.Option[int], len(projects))
	for i, project := range projects {
		options[i] = huh.NewOption(project.Name, project.ID)
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Name").
			Value(&name).
			Validate(components.Required("name")),
		huh.NewSelect[int]().
			Title("Project").
			Options(options...).
			Value(&projectID),
		huh.NewSelect[int]().
			Title("Rate").
			Options(taskRateOptions(rates)...).
			Value(&rateID),
		huh.NewInput().
			Title("Started at (YYYY-MM-DD HH:MM)").
			Value(&startedAt).
			Validate(components.DateTime),
		huh.NewInput().
			Title("Ended at (YYYY-MM-DD HH:MM)").
			Value(&endedAt).
			Validate(components.EntryEndTime(&startedAt, false)),
		huh.NewInput().
			Title("Note").
			Value(&note),
	)).WithShowHelp(true)

	return components.NewForm[taskItem](
		ctx,
		"tasks / add past task",
		form,
		func(ctx context.Context) error {
			started, err := components.ParseDateTime(startedAt)
			if err != nil {
				return err
			}
			ended, err := components.ParseDateTime(endedAt)
			if err != nil {
				return err
			}
			return stor.CreateTaskAndEntry(
				ctx,
				storage.Task{
					Name: strings.TrimSpace(name), ProjectID: projectID,
					RateID: optionalTaskRateID(rateID),
				},
				storage.Entry{
					StartedAt: started,
					EndedAt:   &ended,
					Note:      strings.TrimSpace(note),
				},
			)
		},
	), nil
}

func taskFields(
	task *storage.Task,
	projects []storage.Project,
	rates []storage.Rate,
) (*huh.Form, *taskFormValues, error) {
	if len(projects) == 0 {
		return nil, nil, errors.New("no projects available; create a project first")
	}
	values := &taskFormValues{projectID: projects[0].ID}
	if task != nil {
		values.name = task.Name
		values.projectID = task.ProjectID
		if task.RateID != nil {
			values.rateID = *task.RateID
		}
	}
	options := make([]huh.Option[int], len(projects))
	for i, project := range projects {
		options[i] = huh.NewOption(project.Name, project.ID)
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Name").
			Value(&values.name).
			Validate(components.Required("name")),
		huh.NewSelect[int]().
			Title("Project").
			Options(options...).
			Value(&values.projectID),
		huh.NewSelect[int]().
			Title("Rate").
			Options(taskRateOptions(rates)...).
			Value(&values.rateID),
	)).WithShowHelp(true)
	return form, values, nil
}

func taskRateOptions(rates []storage.Rate) []huh.Option[int] {
	options := []huh.Option[int]{huh.NewOption("Inherit project rate", 0)}
	for _, rate := range rates {
		label := fmt.Sprintf(
			"%s · %s/hour",
			rate.Name,
			components.FormatMoney(int64(rate.AmountMinor), rate.Currency),
		)
		options = append(options, huh.NewOption(label, rate.ID))
	}
	return options
}

func optionalTaskRateID(rateID int) *int {
	if rateID == 0 {
		return nil
	}
	return &rateID
}

type entryItem struct {
	entry storage.Entry
	now   time.Time
	rate  storage.Rate
}

type entryMeta struct {
	task storage.Task
}

func (e entryItem) Title() string {
	endedAt := "active"
	if e.entry.EndedAt != nil {
		endedAt = components.FormatDateTime(*e.entry.EndedAt)
	}
	return components.FormatDateTime(e.entry.StartedAt) + " → " + endedAt
}

func (e entryItem) Description() string {
	endedAt := e.now
	if e.entry.EndedAt != nil {
		endedAt = *e.entry.EndedAt
	}
	description := components.FormatDuration(endedAt.Sub(e.entry.StartedAt))
	if e.rate.ID != 0 {
		description += fmt.Sprintf(
			" · %s %s/hour", e.rate.Name,
			components.FormatMoney(int64(e.rate.AmountMinor), e.rate.Currency),
		)
	} else {
		description += " · Rate unavailable"
	}
	if e.entry.Note != "" {
		description += " · " + e.entry.Note
	}
	return description
}

func (e entryItem) FilterValue() string {
	return e.Title() + " " + e.entry.Note
}

func newEntryPage(
	ctx context.Context,
	stor *storage.Storage,
	taskID int,
	filter *TaskListFilter,
	afterSave func() tea.Cmd,
) components.Page[entryItem] {
	config := components.Config[entryItem]{
		Name:     "time entries",
		Embedded: true,
		Load: func(ctx context.Context) ([]entryItem, any, error) {
			task, err := stor.GetTask(ctx, taskID)
			if err != nil {
				return nil, nil, err
			}
			entries, err := stor.GetEntries(ctx)
			if err != nil {
				return nil, nil, err
			}
			now := time.Now()
			if filter != nil && (filter.Period.Kind != storage.All ||
				!filter.Period.Start.IsZero() || !filter.Period.End.IsZero()) {
				entries = entriesOverlappingPeriod(entries, filter.Period, now)
			}
			rates, err := stor.GetRates(ctx)
			if err != nil {
				return nil, nil, err
			}
			items := entryItems(entries, task.ID, now)
			byID := storage.RatesByID(rates)
			for i := range items {
				if items[i].entry.RateID != nil {
					items[i].rate = byID[*items[i].entry.RateID]
				}
			}
			return items, entryMeta{task: task}, nil
		},
		Create: func(meta any) (*components.Form[entryItem], error) {
			values := meta.(entryMeta)
			return entryForm(ctx, stor, values.task, nil, time.Now()), nil
		},
		Update: func(
			item entryItem,
			meta any,
		) (*components.Form[entryItem], error) {
			values := meta.(entryMeta)
			return entryForm(
				ctx, stor, values.task, &item.entry, time.Now(),
			), nil
		},
		Delete: func(item entryItem) *components.Form[entryItem] {
			return components.NewDeleteForm[entryItem](
				ctx,
				"time entry "+components.FormatDateTime(item.entry.StartedAt),
				func(ctx context.Context) error {
					return stor.DeleteEntry(ctx, item.entry.ID)
				},
			)
		},
		AfterSave: afterSave,
	}
	return components.NewPage(ctx, config)
}

func entriesOverlappingPeriod(
	entries []storage.Entry,
	period storage.Period,
	now time.Time,
) []storage.Entry {
	clipped := storage.EntriesInPeriod(entries, period, now)
	ids := make(map[int]bool, len(clipped))
	for _, entry := range clipped {
		ids[entry.ID] = true
	}
	result := make([]storage.Entry, 0, len(clipped))
	for _, entry := range entries {
		if ids[entry.ID] {
			result = append(result, entry)
		}
	}
	return result
}

func entryItems(
	entries []storage.Entry,
	taskID int,
	now time.Time,
) []entryItem {
	items := make([]entryItem, 0)
	for _, entry := range entries {
		if entry.TaskID != nil && *entry.TaskID == taskID {
			items = append(items, entryItem{entry: entry, now: now})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].entry.StartedAt.After(items[j].entry.StartedAt)
	})
	return items
}

func entryForm(
	ctx context.Context,
	stor *storage.Storage,
	task storage.Task,
	entry *storage.Entry,
	now time.Time,
) *components.Form[entryItem] {
	now = now.Truncate(time.Minute)
	startedAt := components.FormatDateTime(now.Add(-time.Hour))
	endedAt := components.FormatDateTime(now)
	note := ""
	action := "add"
	if entry != nil {
		startedAt = components.FormatDateTime(entry.StartedAt)
		endedAt = ""
		if entry.EndedAt != nil {
			endedAt = components.FormatDateTime(*entry.EndedAt)
		}
		note = entry.Note
		action = "edit"
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Started at (YYYY-MM-DD HH:MM)").
			Value(&startedAt).
			Validate(components.DateTime),
		huh.NewInput().
			Title("Ended at (blank means active)").
			Value(&endedAt).
			Validate(components.EntryEndTime(&startedAt, entry != nil)),
		huh.NewInput().
			Title("Note").
			Value(&note),
	)).WithShowHelp(true)

	return components.NewForm[entryItem](
		ctx,
		"tasks / "+task.Name+" / "+action+" time",
		form,
		func(ctx context.Context) error {
			started, err := components.ParseDateTime(startedAt)
			if err != nil {
				return err
			}
			var ended *time.Time
			if strings.TrimSpace(endedAt) != "" {
				value, err := components.ParseDateTime(endedAt)
				if err != nil {
					return err
				}
				ended = &value
			}
			value := storage.Entry{
				StartedAt: started,
				EndedAt:   ended,
				Note:      strings.TrimSpace(note),
			}
			if entry != nil {
				value.ID = entry.ID
				value.TaskID = entry.TaskID
				value.ProjectID = entry.ProjectID
				value.RateID = entry.RateID
				return stor.UpdateEntry(ctx, value)
			}
			return stor.CreateEntryForTask(
				ctx, task.ID, value.StartedAt, value.EndedAt, value.Note,
			)
		},
	)
}
