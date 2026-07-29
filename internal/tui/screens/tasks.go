package screens

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"chansat/internal/storage"
	"chansat/internal/tui/components"

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
	projects    map[int]string
	tasks       map[int]string
	rates       map[int]storage.Rate
	active      list.Model
	activeItems []storage.Entry
	taskPage    components.Page[taskItem]
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
	if m.taskPage.FormActive() {
		var cmd tea.Cmd
		m.taskPage, cmd = m.taskPage.Update(msg)
		return m, cmd
	}

	var taskCmd tea.Cmd
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg, tea.WindowSizeMsg:
	default:
		m.taskPage, taskCmd = m.taskPage.Update(msg)
		m.resizeTaskPage()
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height
		m.active.SetSize(msg.Width, m.active.Height())
		m.resizeTaskPage()
		return m, nil
	case dashboardLoadedMsg:
		m.now = time.Now()
		m.entries = msg.entries
		m.projectList = msg.projects
		m.projects = projectNames(msg.projects)
		m.tasks = taskNames(msg.tasks)
		m.rates = ratesByID(msg.rates)
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
		if !m.taskPage.GlobalKeysEnabled() {
			m.taskPage, taskCmd = m.taskPage.Update(msg)
			m.resizeTaskPage()
			return m, taskCmd
		}
		switch msg.String() {
		case "/", "n", "e", "enter", "x", "delete", "c":
			m.setFocus(dashboardTaskRow)
			m.taskPage, taskCmd = m.taskPage.Update(msg)
			return m, taskCmd
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
		if !dashboardActionAt(kind, mouse.X) {
			break
		}
		switch kind {
		case dashboardActiveRow:
			m.setFocus(dashboardActiveRow)
			m.active.Select(index)
			return m, pauseEntry(m.ctx, m.stor, m.activeItems[index].ID, m.now)
		case dashboardTaskRow:
			m.setFocus(dashboardTaskRow)
			m.taskPage.Select(index)
			if item, ok := m.taskPage.Selected(); ok {
				return m, startTaskEntry(
					m.ctx, m.stor, item.task, m.projectList, m.now,
				)
			}
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
		return m, tea.Batch(cmd, taskCmd)
	}
	return m, taskCmd
}

func (m Dashboard) View() string {
	if m.taskPage.FormActive() {
		return m.taskPage.View()
	}
	if m.loading {
		return m.spinner.View() + " Loading..."
	}
	if m.err != nil {
		return dashboardErrorStyle.Render("Error: " + m.err.Error())
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
	return m.taskPage.FormActive()
}

func (m Dashboard) GlobalKeysEnabled() bool {
	return m.taskPage.GlobalKeysEnabled()
}

func (m *Dashboard) Reload() tea.Cmd {
	m.loading = true
	return tea.Batch(loadDashboard(m.ctx, m.stor), m.taskPage.Reload())
}

func (m Dashboard) Actions() string {
	actions := m.taskPage.Actions()
	if len(m.activeItems) > 0 {
		actions += "  [space] pause"
	}
	return actions
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
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if entry.TaskID == nil {
			continue
		}
		if entry.EndedAt == nil {
			active[*entry.TaskID] = true
		} else if _, ok := latest[*entry.TaskID]; !ok {
			latest[*entry.TaskID] = entry
		}
	}

	byID := make(map[int]storage.Task, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	ordered := make([]storage.Task, 0, len(tasks))
	seen := make(map[int]bool)
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if entry.TaskID == nil || active[*entry.TaskID] || seen[*entry.TaskID] {
			continue
		}
		if task, ok := byID[*entry.TaskID]; ok {
			ordered = append(ordered, task)
			seen[task.ID] = true
		}
	}
	for _, task := range tasks {
		if !active[task.ID] && !seen[task.ID] {
			ordered = append(ordered, task)
		}
	}

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
	amounts := make(map[string]int64)
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
		amounts[rate.Currency] += int64(rate.AmountMinor) * seconds / 3600
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
