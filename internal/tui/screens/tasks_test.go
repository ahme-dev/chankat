package screens

import (
	"strings"
	"testing"
	"time"

	"chansat/internal/storage"

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

	if got := m.entryAmount(entry, time.Now()); got != "0.00 USD" {
		t.Fatalf("got amount %q, want zero", got)
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
