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
	ctx         context.Context
	stor        *storage.Storage
	entries     []storage.Entry
	projectList []storage.Project
	taskList    []storage.Task
	projects    map[int]string
	tasks       map[int]string
	rates       map[int]storage.Rate
	active      list.Model
	activeItems []storage.Entry
	taskPage    components.Page[taskItem]
	entryPage   *components.Page[entryItem]
	detailTask  *storage.Task
	focus       dashboardRow
	spinner     spinner.Model
	now         time.Time
	err         error
	loading     bool
	viewport    viewport.Model
}

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

type entryPausedMsg struct{}

type entryPauseFailedMsg struct {
	err error
}

type entryStartedMsg struct{}

type entryStartFailedMsg struct {
	err error
}

var (
	dashboardSectionStyle = list.DefaultStyles().StatusBar
	dashboardMutedStyle   = list.DefaultStyles().NoItems.PaddingLeft(2)
	dashboardErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

type dashboardItem struct {
	title       string
	description string
}

func (i dashboardItem) Title() string       { return i.title }
func (i dashboardItem) Description() string { return i.description }
func (i dashboardItem) FilterValue() string { return i.title + " " + i.description }

func NewDashboard(ctx context.Context, stor *storage.Storage) Dashboard {
	m := Dashboard{
		ctx:      ctx,
		stor:     stor,
		active:   newDashboardList(true),
		focus:    dashboardActiveRow,
		spinner:  spinner.New(),
		viewport: viewport.New(0, 0),
		now:      time.Now(),
		loading:  true,
	}
	m.taskPage = newTaskPage(ctx, stor, func() tea.Cmd {
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
	if _, ok := msg.(dashboardTickMsg); ok {
		m.now = time.Time(msg.(dashboardTickMsg))
		m.refreshTables()
		return m, tickDashboard()
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
	case dashboardFailedMsg:
		m.loading = false
		m.err = msg.err
	case entryPausedMsg:
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
	case entryPauseFailedMsg:
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
					return m, pauseEntry(m.ctx, m.stor, m.activeItems[cursor].ID, m.now)
				}
			case dashboardTaskRow:
				if item, ok := m.taskPage.Selected(); ok {
					return m, startTaskEntry(
						m.ctx, m.stor, item.task, m.projectList, m.now,
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
				return m, pauseEntry(m.ctx, m.stor, m.activeItems[index].ID, m.now)
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
					m.ctx, m.stor, item.task, m.projectList, m.now,
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
	active := make([]list.Item, 0)
	m.activeItems = m.activeItems[:0]
	for _, entry := range m.entries {
		if entry.EndedAt == nil {
			m.activeItems = append(m.activeItems, entry)
			duration := m.now.Sub(entry.StartedAt)
			if duration < 0 {
				duration = 0
			}
			amounts := make(map[string]int64)
			if entry.TaskID != nil {
				duration, amounts = taskTotals(
					m.entries,
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
				"session " + components.FormatDuration(session),
				"total " + components.FormatDuration(duration),
			}
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
	var y int
	switch m.focus {
	case dashboardActiveRow:
		if len(m.activeItems) == 0 {
			return
		}
		y = 2 + m.active.Index()*2
	case dashboardTaskRow:
		if m.taskPage.VisibleCount() == 0 {
			return
		}
		activeBodyHeight := 1
		if len(m.activeItems) > 0 {
			activeBodyHeight = len(m.activeItems) * 2
		}
		y = 3 + activeBodyHeight + 2 + m.taskPage.Index()*2
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
	activeBodyHeight := 1
	if len(m.activeItems) > 0 {
		activeBodyHeight = len(m.activeItems) * 2
		if offset := y - 2; offset >= 0 && offset%2 == 0 {
			if index := offset / 2; index < len(m.activeItems) {
				return dashboardActiveRow, index
			}
		}
	}

	taskTitle := 3 + activeBodyHeight
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

func pauseEntry(
	ctx context.Context,
	stor *storage.Storage,
	entryID int,
	endedAt time.Time,
) tea.Cmd {
	return func() tea.Msg {
		if err := stor.PauseEntry(ctx, entryID, endedAt); err != nil {
			return entryPauseFailedMsg{err: err}
		}
		return entryPausedMsg{}
	}
}

func startTaskEntry(
	ctx context.Context,
	stor *storage.Storage,
	task storage.Task,
	projects []storage.Project,
	startedAt time.Time,
) tea.Cmd {
	return func() tea.Msg {
		var rateID int
		for _, project := range projects {
			if project.ID == task.ProjectID {
				rateID = project.RateID
				break
			}
		}
		taskID := task.ID
		projectID := task.ProjectID
		if err := stor.CreateEntry(ctx, storage.Entry{
			TaskID:    &taskID,
			ProjectID: &projectID,
			RateID:    &rateID,
			StartedAt: startedAt,
		}); err != nil {
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
	return m.taskPage.FormActive() ||
		m.entryPage != nil && m.entryPage.FormActive()
}

func (m Dashboard) GlobalKeysEnabled() bool {
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
		"[enter] details  [e] edit task  [x/delete] delete  " +
		"[space] start/pause"
}

func (m *Dashboard) resizeTaskPage() {
	activeBodyHeight := len(m.activeItems) * 2
	if activeBodyHeight < 1 {
		activeBodyHeight = 1
	}
	height := m.viewport.Height - activeBodyHeight - 6
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
		form, err := taskForm(m.ctx, m.stor, &item.task, m.projectList)
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
	page := newEntryPage(m.ctx, m.stor, task.ID, func() tea.Cmd {
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
		form = components.NewDeleteForm[taskItem](
			m.ctx,
			task.Name,
			func(ctx context.Context) error {
				return m.stor.DeleteTask(ctx, task.ID)
			},
		)
	} else {
		var err error
		form, err = taskForm(m.ctx, m.stor, &task, m.projectList)
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
	form, err := historicalTaskForm(m.ctx, m.stor, m.projectList, m.now)
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
			return task != previous
		}
	}
	m.detailTask = nil
	m.entryPage = nil
	return true
}

func (m Dashboard) detailView() string {
	task := *m.detailTask
	project := m.projects[task.ProjectID]
	duration, amounts := taskTotals(m.entries, m.rates, task.ID, m.now)
	summary := project + " · total " + components.FormatDuration(duration)
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
	description string
}

func (t taskItem) Title() string       { return "[>] " + t.task.Name }
func (t taskItem) Description() string { return t.description }
func (t taskItem) FilterValue() string {
	return t.task.Name + " " + t.project.Name
}

type taskFormValues struct {
	name      string
	projectID int
}

func newTaskPage(
	ctx context.Context,
	stor *storage.Storage,
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
			return taskItems(tasks, projects, entries, rates), projects, nil
		},
		Create: func(meta any) (*components.Form[taskItem], error) {
			return taskForm(ctx, stor, nil, meta.([]storage.Project))
		},
		Update: func(item taskItem, meta any) (*components.Form[taskItem], error) {
			return taskForm(ctx, stor, &item.task, meta.([]storage.Project))
		},
		Delete: func(item taskItem) *components.Form[taskItem] {
			return components.NewDeleteForm[taskItem](
				ctx,
				item.task.Name,
				func(ctx context.Context) error {
					return stor.DeleteTask(ctx, item.task.ID)
				},
			)
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
	projectsByID := make(map[int]storage.Project, len(projects))
	for _, project := range projects {
		projectsByID[project.ID] = project
	}
	ratesByID := make(map[int]storage.Rate, len(rates))
	for _, rate := range rates {
		ratesByID[rate.ID] = rate
	}
	active := make(map[int]bool)
	latest := make(map[int]storage.Entry)
	for _, entry := range entries {
		if entry.TaskID == nil {
			continue
		}
		if entry.EndedAt == nil {
			active[*entry.TaskID] = true
			continue
		}
		previous, ok := latest[*entry.TaskID]
		if !ok ||
			entry.EndedAt.After(*previous.EndedAt) ||
			entry.EndedAt.Equal(*previous.EndedAt) && entry.ID > previous.ID {
			latest[*entry.TaskID] = entry
		}
	}

	ordered := make([]storage.Task, 0, len(tasks))
	for _, task := range tasks {
		if !active[task.ID] {
			ordered = append(ordered, task)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		left, leftOK := latest[ordered[i].ID]
		right, rightOK := latest[ordered[j].ID]
		if leftOK != rightOK {
			return leftOK
		}
		if !leftOK {
			return false
		}
		if !left.EndedAt.Equal(*right.EndedAt) {
			return left.EndedAt.After(*right.EndedAt)
		}
		return left.ID > right.ID
	})

	items := make([]taskItem, 0, len(ordered))
	for _, task := range ordered {
		project := projectsByID[task.ProjectID]
		description := project.Name
		if entry, ok := latest[task.ID]; ok {
			duration, amounts := taskTotals(
				entries,
				ratesByID,
				task.ID,
				time.Time{},
			)
			description += " · total " + components.FormatDuration(duration)
			if amount := formatTaskAmounts(amounts); amount != "" {
				description += " · " + amount + " earned"
			}
			description += " · worked " + components.FormatDate(*entry.EndedAt)
		}
		items = append(items, taskItem{
			task: task, project: project, description: description,
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
	var duration time.Duration
	minorSeconds := make(map[string]int64)
	for _, entry := range entries {
		if entry.TaskID == nil || *entry.TaskID != taskID {
			continue
		}
		endedAt := now
		if entry.EndedAt != nil {
			endedAt = *entry.EndedAt
		}
		elapsed := endedAt.Sub(entry.StartedAt)
		if elapsed < 0 {
			elapsed = 0
		}
		duration += elapsed
		if entry.RateID == nil {
			continue
		}
		rate, ok := rates[*entry.RateID]
		if !ok {
			continue
		}
		seconds := int64(elapsed / time.Second)
		minorSeconds[rate.Currency] += int64(rate.AmountMinor) * seconds
	}
	amounts := make(map[string]int64, len(minorSeconds))
	for currency, total := range minorSeconds {
		amounts[currency] = total / 3600
	}
	return duration, amounts
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

func taskForm(
	ctx context.Context,
	stor *storage.Storage,
	task *storage.Task,
	projects []storage.Project,
) (*components.Form[taskItem], error) {
	form, values, err := taskFields(task, projects)
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
			}
			if task == nil {
				return stor.CreateTaskAndStart(ctx, value, time.Now())
			}
			value.ID = task.ID
			return stor.UpdateTask(ctx, value)
		},
	), nil
}

func historicalTaskForm(
	ctx context.Context,
	stor *storage.Storage,
	projects []storage.Project,
	now time.Time,
) (*components.Form[taskItem], error) {
	if len(projects) == 0 {
		return nil, errors.New("no projects available; create a project first")
	}
	now = now.Truncate(time.Minute)
	name := ""
	projectID := projects[0].ID
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
) (*huh.Form, *taskFormValues, error) {
	if len(projects) == 0 {
		return nil, nil, errors.New("no projects available; create a project first")
	}
	values := &taskFormValues{projectID: projects[0].ID}
	if task != nil {
		values.name = task.Name
		values.projectID = task.ProjectID
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
	)).WithShowHelp(true)
	return form, values, nil
}

type entryItem struct {
	entry storage.Entry
	now   time.Time
}

type entryMeta struct {
	task    storage.Task
	project storage.Project
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
			project, err := stor.GetProject(ctx, task.ProjectID)
			if err != nil {
				return nil, nil, err
			}
			now := time.Now()
			return entryItems(entries, task.ID, now), entryMeta{
				task: task, project: project,
			}, nil
		},
		Create: func(meta any) (*components.Form[entryItem], error) {
			values := meta.(entryMeta)
			return entryForm(
				ctx, stor, values.task, values.project, nil, time.Now(),
			), nil
		},
		Update: func(
			item entryItem,
			meta any,
		) (*components.Form[entryItem], error) {
			values := meta.(entryMeta)
			return entryForm(
				ctx, stor, values.task, values.project, &item.entry, time.Now(),
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
	project storage.Project,
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
			value.TaskID = &task.ID
			value.ProjectID = &project.ID
			value.RateID = &project.RateID
			return stor.CreateEntry(ctx, value)
		},
	)
}
