package tui

import (
	"strings"
	"testing"
	"time"

	"chankat/internal/storage"
	"chankat/internal/tui/screens"
)

func TestAdjacentTab(t *testing.T) {
	t.Run("next", func(t *testing.T) {
		if got := adjacentTab(tasksScreen, 1); got != projectsScreen {
			t.Fatalf("got screen %d, want projects", got)
		}
	})

	t.Run("previous", func(t *testing.T) {
		if got := adjacentTab(tasksScreen, -1); got != dashboardScreen {
			t.Fatalf("got screen %d, want dashboard", got)
		}
	})

	t.Run("dashboard previous wraps", func(t *testing.T) {
		if got := adjacentTab(dashboardScreen, -1); got != paymentsScreen {
			t.Fatalf("got screen %d, want payments", got)
		}
	})
}

func TestDashboardFillsContentHeight(t *testing.T) {
	m := model{
		active: tasksScreen,
		height: 20,
	}

	lines := strings.Split(m.View(), "\n")
	if got := len(lines); got != m.height {
		t.Fatalf("got %d lines, want %d", got, m.height)
	}
	if got := lines[len(lines)-1]; got !=
		"[/] search  [n] new & track  [a] add past task  [enter] details  "+
			"[e] edit task  [x/delete] archive  [space] start/pause  "+
			"[f] filters  [F] reset filters  "+
			"[shift+up/down or K/J] period  "+
			"[shift+left/right or H/L] move" {
		t.Fatalf("got final line %q", got)
	}
}

func TestDashboardProjectOpensFilteredTasksTab(t *testing.T) {
	period, err := storage.CurrentPeriod(
		storage.Week,
		time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(t.Context(), nil)
	updated, cmd := m.Update(screens.OpenTasksMsg{ProjectID: 7, Period: period})
	result := updated.(model)
	if result.active != tasksScreen {
		t.Fatalf("active screen = %d, want tasks", result.active)
	}
	if cmd == nil {
		t.Fatal("task filter did not trigger a refresh")
	}
}
