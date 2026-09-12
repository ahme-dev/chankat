package screens

import (
	"strings"
	"testing"
	"time"

	"chankat/internal/storage"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDashboardTableTransitions(t *testing.T) {
	m := NewDashboard(t.Context(), nil)
	m.loading = false
	m.now = time.Unix(1_700_000_060, 0)

	m.refreshTables()

	m.entries = []storage.Entry{{
		ID:        1,
		StartedAt: time.Unix(1_700_000_000, 0),
	}}
	m.refreshTables()

	if got := len(m.active.Items()); got != 1 {
		t.Fatalf("got %d active rows, want 1", got)
	}
	if m.active.Index() != 0 {
		t.Fatalf("got cursor %d, want 0", m.active.Index())
	}
	if !strings.Contains(m.View(), "0h 01m") {
		t.Fatalf("dashboard does not show elapsed time:\n%s", m.View())
	}

	m.entries = nil
	m.refreshTables()

	if got := len(m.active.Items()); got != 0 {
		t.Fatalf("got %d active rows, want 0", got)
	}
}

func TestDashboardTasksByRecentEntry(t *testing.T) {
	taskOne := 1
	taskTwo := 2
	projectID := 1
	ended := time.Now()
	tasks := []storage.Task{
		{ID: taskOne, Name: "one", ProjectID: projectID},
		{ID: taskTwo, Name: "two", ProjectID: projectID},
	}
	entries := []storage.Entry{
		{ID: 1, TaskID: &taskOne, ProjectID: &projectID, StartedAt: ended.Add(-time.Hour), EndedAt: &ended},
		{ID: 2, TaskID: &taskTwo, ProjectID: &projectID, StartedAt: ended.Add(-time.Hour), EndedAt: &ended},
		{ID: 3, TaskID: &taskOne, ProjectID: &projectID, StartedAt: ended.Add(-time.Hour), EndedAt: &ended},
	}
	items := taskItems(
		tasks,
		[]storage.Project{{ID: projectID, Name: "project"}},
		entries,
		nil,
	)

	if got := len(items); got != 2 {
		t.Fatalf("got %d tasks, want 2", got)
	}
	if got := items[0].Title(); got != "[>] one" {
		t.Fatalf("got first recent task %q, want task one", got)
	}
	if got := items[0].Description(); !strings.Contains(got, "total 2h 00m") {
		t.Fatalf("got description %q, want cumulative duration", got)
	}
}

func TestTaskItemsDisplayEffectiveRate(t *testing.T) {
	overrideID := 2
	items := taskItems(
		[]storage.Task{
			{ID: 1, Name: "inherited", ProjectID: 1},
			{ID: 2, Name: "overridden", ProjectID: 1, RateID: &overrideID},
		},
		[]storage.Project{{ID: 1, Name: "project", RateID: 1}},
		nil,
		[]storage.Rate{
			{ID: 1, Name: "standard", AmountMinor: 10_000, Currency: "USD"},
			{ID: 2, Name: "special", AmountMinor: 15_000, Currency: "USD"},
		},
	)
	if len(items) != 2 ||
		!strings.Contains(items[0].Description(), "$100.00/hour (project rate)") ||
		!strings.Contains(items[1].Description(), "$150.00/hour (task rate)") {
		t.Fatalf("task rates not displayed: %#v", items)
	}
}

func TestHistoricalTaskDisplaysUsedRateSeparatelyFromNextRate(t *testing.T) {
	taskID, projectID, oldID := 1, 1, 1
	end := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	rates := []storage.Rate{
		{ID: oldID, Name: "Old", AmountMinor: 10000, Currency: "USD"},
		{ID: 2, Name: "New", AmountMinor: 20000, Currency: "USD"},
	}
	entry := storage.Entry{TaskID: &taskID, ProjectID: &projectID, RateID: &oldID,
		StartedAt: end.Add(-time.Hour), EndedAt: &end}
	items := taskItems([]storage.Task{{ID: taskID, Name: "Historical", ProjectID: projectID}},
		[]storage.Project{{ID: projectID, Name: "Client", RateID: 2}}, []storage.Entry{entry}, rates)
	for _, want := range []string{"Used: Old $100.00/hour", "Next rate: New", "$100.00 earned"} {
		if !strings.Contains(items[0].Description(), want) {
			t.Fatalf("missing %q: %s", want, items[0].Description())
		}
	}
	row := entryItem{entry: entry, now: end, rate: rates[0]}
	if !strings.Contains(row.Description(), "Old $100.00/hour") {
		t.Fatalf("entry rate = %s", row.Description())
	}
}

func TestDashboardHistoricalEntryDoesNotBecomeLatest(t *testing.T) {
	taskOne := 1
	taskTwo := 2
	projectID := 1
	now := time.Now()
	older := now.Add(-7 * 24 * time.Hour)
	tasks := []storage.Task{
		{ID: taskOne, Name: "one", ProjectID: projectID},
		{ID: taskTwo, Name: "two", ProjectID: projectID},
	}
	entries := []storage.Entry{
		{
			ID: 1, TaskID: &taskOne,
			StartedAt: now.Add(-time.Hour), EndedAt: &now,
		},
		{
			ID: 2, TaskID: &taskTwo,
			StartedAt: older.Add(-time.Hour), EndedAt: &older,
		},
	}

	items := taskItems(
		tasks,
		[]storage.Project{{ID: projectID, Name: "project"}},
		entries,
		nil,
	)
	if got := items[0].task.ID; got != taskOne {
		t.Fatalf("got first task %d, want task with latest worked time", got)
	}
}

func TestTaskItemsForProjectAndPeriod(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	period, err := storage.CurrentPeriod(storage.Day, now)
	if err != nil {
		t.Fatal(err)
	}
	projectOne, projectTwo := 1, 2
	taskOne, taskTwo := 10, 20
	firstEnd := period.Start.Add(2 * time.Hour)
	secondEnd := period.Start.Add(3 * time.Hour)
	items := taskItemsForFilter(
		[]storage.Task{
			{ID: taskOne, Name: "one", ProjectID: projectOne},
			{ID: taskTwo, Name: "two", ProjectID: projectTwo},
		},
		[]storage.Project{
			{ID: projectOne, Name: "first"},
			{ID: projectTwo, Name: "second"},
		},
		[]storage.Entry{
			{ID: 1, TaskID: &taskOne, ProjectID: &projectOne,
				StartedAt: period.Start.Add(-time.Hour), EndedAt: &firstEnd},
			{ID: 2, TaskID: &taskTwo, ProjectID: &projectTwo,
				StartedAt: period.Start.Add(time.Hour), EndedAt: &secondEnd},
		},
		nil,
		TaskListFilter{ProjectID: projectOne, Period: period},
		now,
	)

	if len(items) != 1 || items[0].task.ID != taskOne {
		t.Fatalf("unexpected filtered tasks: %#v", items)
	}
	if got := items[0].Description(); !strings.Contains(got, "period 2h 00m") {
		t.Fatalf("got description %q, want clipped period total", got)
	}
}

func TestDashboardFilterAdvancesWithCurrentPeriod(t *testing.T) {
	before := time.Date(2026, 8, 9, 23, 59, 59, 0, time.UTC)
	after := before.Add(2 * time.Second)
	period, err := storage.CurrentPeriod(storage.Day, before)
	if err != nil {
		t.Fatal(err)
	}
	m := NewDashboard(t.Context(), nil)
	m.now = before
	m.filter.Period = period

	if changed := m.updateFilterNow(after); !changed {
		t.Fatal("current filter did not advance")
	}
	if !m.filter.Period.Start.Equal(time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("period start = %s", m.filter.Period.Start)
	}
}

func TestEntriesForUnassignedProject(t *testing.T) {
	projectID := 1
	entries := []storage.Entry{{ID: 1}, {ID: 2, ProjectID: &projectID}}
	filtered := entriesForProject(entries, 0)
	if len(filtered) != 1 || filtered[0].ID != 1 {
		t.Fatalf("unexpected unassigned entries: %#v", filtered)
	}
}

func TestDashboardApplyFilterUpdatesTaskRows(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	period, err := storage.CurrentPeriod(storage.Day, now)
	if err != nil {
		t.Fatal(err)
	}
	projectOne, projectTwo := 1, 2
	taskOne, taskTwo := 10, 20
	end := now.Add(-time.Hour)
	m := NewDashboard(t.Context(), nil)
	m.now = now
	m.loading = false
	m.projectList = []storage.Project{
		{ID: projectOne, Name: "first"},
		{ID: projectTwo, Name: "second"},
	}
	m.taskList = []storage.Task{
		{ID: taskOne, Name: "one", ProjectID: projectOne},
		{ID: taskTwo, Name: "two", ProjectID: projectTwo},
	}
	m.projects = projectNames(m.projectList)
	m.entries = []storage.Entry{
		{ID: 1, TaskID: &taskOne, ProjectID: &projectOne,
			StartedAt: end.Add(-time.Hour), EndedAt: &end},
		{ID: 2, TaskID: &taskTwo, ProjectID: &projectTwo,
			StartedAt: end.Add(-time.Hour), EndedAt: &end},
	}

	m.ApplyFilter(projectTwo, period)
	selected, ok := m.taskPage.Selected()
	if !ok || m.taskPage.VisibleCount() != 1 || selected.task.ID != taskTwo {
		t.Fatalf("unexpected filtered task: %#v", selected)
	}
	if got := m.filterLabel(); !strings.Contains(got, "second") ||
		!strings.Contains(got, period.Label()) {
		t.Fatalf("filter label = %q", got)
	}
}

func TestTaskFilterMenuKeepsProjectSelection(t *testing.T) {
	m := NewDashboard(t.Context(), nil)
	m.projectList = []storage.Project{{ID: 7, Name: "project"}}
	m, _ = m.openFilterMenu()
	*m.filterProjectID = 7
	m.applyFilterDraft()
	if m.filter.ProjectID != 7 {
		t.Fatalf("project filter = %d", m.filter.ProjectID)
	}
}

func TestDashboardResetFilter(t *testing.T) {
	m := NewDashboard(t.Context(), nil)
	period, err := storage.CurrentPeriod(
		storage.Week,
		time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	m.filter.ProjectID = 7
	m.filter.Period = period

	updated, cmd := m.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'F'},
	})
	if cmd == nil {
		t.Fatal("reset did not refresh tasks")
	}
	if updated.filter.ProjectID != allProjectsFilter ||
		updated.filter.Period.Kind != storage.All {
		t.Fatalf("filter was not reset: %#v", updated.filter)
	}
}

func TestDashboardResumedTaskTotals(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	previousEnd := now.Add(-30 * time.Minute)
	taskID := 1
	projectID := 1
	rateID := 1
	m := NewDashboard(t.Context(), nil)
	m.loading = false
	m.now = now
	m.tasks = map[int]string{taskID: "task"}
	m.projects = map[int]string{projectID: "project"}
	m.rates = map[int]storage.Rate{
		rateID: {ID: rateID, AmountMinor: 5000, Currency: "USD"},
	}
	m.entries = []storage.Entry{
		{
			ID: 1, TaskID: &taskID, ProjectID: &projectID, RateID: &rateID,
			StartedAt: now.Add(-90 * time.Minute), EndedAt: &previousEnd,
		},
		{
			ID: 2, TaskID: &taskID, ProjectID: &projectID, RateID: &rateID,
			StartedAt: previousEnd,
		},
	}

	m.refreshTables()

	view := m.View()
	if !strings.Contains(view, "1h 30m") {
		t.Fatalf("dashboard does not show cumulative duration:\n%s", view)
	}
	if !strings.Contains(view, "$75.00 earned") {
		t.Fatalf("dashboard does not show cumulative amount:\n%s", view)
	}
	if !strings.Contains(view, "$50.00/hour") {
		t.Fatalf("dashboard does not show active rate:\n%s", view)
	}
}

func TestDashboardRowAt(t *testing.T) {
	m := NewDashboard(t.Context(), nil)
	taskID := 1
	m.taskPage.SetItems([]taskItem{{
		task: storage.Task{ID: 2, Name: "inactive", ProjectID: 1},
	}})
	m.entries = []storage.Entry{{
		ID:        1,
		TaskID:    &taskID,
		StartedAt: time.Now(),
	}}
	m.refreshTables()

	t.Run("active entry", func(t *testing.T) {
		kind, index := m.rowAt(4)
		if kind != dashboardActiveRow || index != 0 {
			t.Fatalf("got (%d, %d), want active row 0", kind, index)
		}
	})

	t.Run("available task", func(t *testing.T) {
		kind, index := m.rowAt(9)
		if kind != dashboardTaskRow || index != 0 {
			t.Fatalf("got (%d, %d), want task row 0", kind, index)
		}
	})
}

func TestDashboardActionAt(t *testing.T) {
	t.Run("icon", func(t *testing.T) {
		if !dashboardActionAt(dashboardActiveRow, 2) ||
			!dashboardActionAt(dashboardActiveRow, 5) {
			t.Fatal("active icon is not clickable")
		}
		if !dashboardActionAt(dashboardTaskRow, 2) ||
			!dashboardActionAt(dashboardTaskRow, 4) {
			t.Fatal("task icon is not clickable")
		}
	})

	t.Run("outside icon", func(t *testing.T) {
		if dashboardActionAt(dashboardActiveRow, 1) ||
			dashboardActionAt(dashboardActiveRow, 6) ||
			dashboardActionAt(dashboardTaskRow, 5) {
			t.Fatal("area outside icon is clickable")
		}
	})
}

func TestDashboardKeyboardNavigation(t *testing.T) {
	activeTaskID := 1
	m := NewDashboard(t.Context(), nil)
	m.loading = false
	m.projectList = []storage.Project{{ID: 1, RateID: 1}}
	m.taskPage.SetItems([]taskItem{{
		task: storage.Task{ID: 2, Name: "available", ProjectID: 1},
	}})
	m.entries = []storage.Entry{{
		ID:        1,
		TaskID:    &activeTaskID,
		StartedAt: time.Now(),
	}}
	m.refreshTables()

	t.Run("moves between sections", func(t *testing.T) {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		if updated.focus != dashboardTaskRow {
			t.Fatalf("got focus %d, want available tasks", updated.focus)
		}

		updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		if updated.focus != dashboardActiveRow {
			t.Fatalf("got focus %d, want active entries", updated.focus)
		}
	})

	t.Run("space activates selection", func(t *testing.T) {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		_, cmd := updated.Update(tea.KeyMsg{Type: tea.KeySpace})
		if cmd == nil {
			t.Fatal("space did not start selected task")
		}
	})
}

func TestPauseTaskStopsAllTaskEntries(t *testing.T) {
	t.Setenv("CHANKAT_DATA_PATH", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	stor, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stor.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := stor.Migrate(); err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	if err := stor.CreateRate(ctx, storage.Rate{
		Name: "rate", AmountMinor: 100, Currency: "USD",
	}); err != nil {
		t.Fatal(err)
	}
	if err := stor.CreateProject(ctx, storage.Project{
		Name: "project", RateID: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := stor.CreateTask(ctx, storage.Task{
		Name: "task", ProjectID: 1,
	}); err != nil {
		t.Fatal(err)
	}
	startedAt := time.Unix(1_700_000_000, 0)
	for range 2 {
		if err := stor.StartTask(ctx, 1, startedAt); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := stor.GetActiveEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	msg := pauseTask(ctx, stor, entries[0], startedAt.Add(time.Hour))()
	if _, ok := msg.(taskPausedMsg); !ok {
		t.Fatalf("unexpected pause result: %#v", msg)
	}
	entries, err = stor.GetActiveEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("%d entries remain active", len(entries))
	}
}

func TestDashboardEntryAmount(t *testing.T) {
	rateID := 1
	m := NewDashboard(t.Context(), nil)
	m.rates = map[int]storage.Rate{
		rateID: {ID: rateID, AmountMinor: 5000, Currency: "USD"},
	}
	entry := storage.Entry{
		RateID:    &rateID,
		StartedAt: time.Now().Add(time.Second),
	}

	if got := m.entryAmount(entry, time.Now()); got != "$0.00" {
		t.Fatalf("got amount %q, want zero", got)
	}
}

func TestTaskTotalsRoundAfterAggregation(t *testing.T) {
	taskID := 1
	rateID := 1
	startedAt := time.Unix(1_700_000_000, 0)
	firstEnd := startedAt.Add(30 * time.Minute)
	secondEnd := firstEnd.Add(30 * time.Minute)
	entries := []storage.Entry{
		{
			TaskID: &taskID, RateID: &rateID,
			StartedAt: startedAt, EndedAt: &firstEnd,
		},
		{
			TaskID: &taskID, RateID: &rateID,
			StartedAt: firstEnd, EndedAt: &secondEnd,
		},
	}
	rates := map[int]storage.Rate{
		rateID: {ID: rateID, AmountMinor: 1, Currency: "USD"},
	}

	_, amounts := taskTotals(entries, rates, taskID, secondEnd)
	if got := amounts["USD"]; got != 1 {
		t.Fatalf("got %d minor units, want 1", got)
	}
}

func TestDashboardViewportHeight(t *testing.T) {
	m := NewDashboard(t.Context(), nil)
	m.loading = false
	m.refreshTables()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 5})

	if got := len(strings.Split(m.View(), "\n")); got != 5 {
		t.Fatalf("got dashboard height %d, want 5", got)
	}
}

func TestDashboardTaskDetail(t *testing.T) {
	task := storage.Task{ID: 1, Name: "task", ProjectID: 2}
	m := NewDashboard(t.Context(), nil)
	m.loading = false
	m.focus = dashboardTaskRow
	m.projectList = []storage.Project{{ID: 2, Name: "project", RateID: 3}}
	m.taskList = []storage.Task{task}
	m.projects = map[int]string{2: "project"}
	m.taskPage.SetItems([]taskItem{{
		task: task, project: m.projectList[0],
	}})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.detailTask == nil || updated.detailTask.ID != task.ID {
		t.Fatal("enter did not open task details")
	}
	if updated.entryPage == nil || cmd == nil {
		t.Fatal("task details did not initialize the entry list")
	}
	if !strings.Contains(updated.Actions(), "[n] add time") {
		t.Fatalf("unexpected detail actions: %q", updated.Actions())
	}

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.detailTask != nil || updated.entryPage != nil {
		t.Fatal("escape did not close task details")
	}
}

func TestRefreshDetailTaskComparesRateValues(t *testing.T) {
	firstRateID, reloadedRateID := 3, 3
	m := NewDashboard(t.Context(), nil)
	m.detailTask = &storage.Task{
		ID: 1, Name: "task", ProjectID: 2, RateID: &firstRateID,
	}
	m.taskList = []storage.Task{{
		ID: 1, Name: "task", ProjectID: 2, RateID: &reloadedRateID,
	}}
	if changed := m.refreshDetailTask(); changed {
		t.Fatal("equal rate values were treated as a task change")
	}

	changedRateID := 4
	m.taskList[0].RateID = &changedRateID
	if changed := m.refreshDetailTask(); !changed {
		t.Fatal("changed rate was not detected")
	}
}

func TestDashboardEditsActiveTask(t *testing.T) {
	task := storage.Task{ID: 1, Name: "active", ProjectID: 2}
	taskID := task.ID
	m := NewDashboard(t.Context(), nil)
	m.loading = false
	m.projectList = []storage.Project{{ID: 2, Name: "project", RateID: 3}}
	m.taskList = []storage.Task{task}
	m.entries = []storage.Entry{{
		ID: 4, TaskID: &taskID, StartedAt: time.Now(),
	}}
	m.refreshTables()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if !updated.taskPage.FormActive() || cmd == nil {
		t.Fatal("edit did not open the active task form")
	}
}

func TestEntryItems(t *testing.T) {
	taskID := 1
	otherTaskID := 2
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.Local)
	earlierEnd := now.Add(-time.Hour)
	entries := []storage.Entry{
		{
			ID: 1, TaskID: &taskID,
			StartedAt: now.Add(-2 * time.Hour), EndedAt: &earlierEnd,
			Note: "earlier",
		},
		{
			ID: 2, TaskID: &otherTaskID,
			StartedAt: now.Add(-30 * time.Minute),
		},
		{
			ID: 3, TaskID: &taskID,
			StartedAt: now.Add(-15 * time.Minute),
		},
	}

	items := entryItems(entries, taskID, now)
	if len(items) != 2 || items[0].entry.ID != 3 || items[1].entry.ID != 1 {
		t.Fatalf("unexpected entry order: %#v", items)
	}
	if !strings.Contains(items[0].Title(), "active") {
		t.Fatalf("active entry title missing status: %q", items[0].Title())
	}
	if !strings.Contains(items[1].Description(), "earlier") {
		t.Fatalf("entry description missing note: %q", items[1].Description())
	}
}
