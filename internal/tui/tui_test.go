package tui

import (
	"strings"
	"testing"
)

func TestAdjacentTab(t *testing.T) {
	t.Run("next", func(t *testing.T) {
		if got := adjacentTab(tasksScreen, 1); got != projectsScreen {
			t.Fatalf("got screen %d, want projects", got)
		}
	})

	t.Run("previous wraps", func(t *testing.T) {
		if got := adjacentTab(tasksScreen, -1); got != paymentsScreen {
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
		"[/] search  [n] new  [e/enter] edit  [x/delete] delete  [c] copy" {
		t.Fatalf("got final line %q", got)
	}
}
