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
		kind, index := m.rowAt(2)
		if kind != dashboardActiveRow || index != 0 {
			t.Fatalf("got (%d, %d), want active row 0", kind, index)
		}
	})

	t.Run("available task", func(t *testing.T) {
		kind, index := m.rowAt(7)
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
