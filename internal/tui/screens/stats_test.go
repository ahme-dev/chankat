package screens

import (
	"strings"
	"testing"
	"time"

	"chankat/internal/storage"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStatsPeriodKeysAndProjectNavigation(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	period, err := storage.CurrentPeriod(storage.Day, now)
	if err != nil {
		t.Fatal(err)
	}
	project1, project2, taskID, rateID := 1, 2, 1, 1
	end := now.Add(-time.Hour)
	m := NewStats(t.Context(), nil)
	m.now, m.period, m.loading = now, period, false
	m.width, m.height = 80, 20
	m.projects = []storage.Project{
		{ID: 1, Name: "Acme", RateID: 1},
		{ID: 2, Name: "Beta", RateID: 1},
	}
	m.tasks = []storage.Task{{ID: 1, Name: "Build", ProjectID: 1}}
	m.rates = []storage.Rate{{ID: 1, AmountMinor: 10_000, Currency: "USD"}}
	m.entries = []storage.Entry{
		{TaskID: &taskID, ProjectID: &project1, RateID: &rateID,
			StartedAt: now.Add(-3 * time.Hour), EndedAt: &end},
		{TaskID: &taskID, ProjectID: &project2, RateID: &rateID,
			StartedAt: now.Add(-time.Hour)},
	}
	m.refresh()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.list.Index() != 1 {
		t.Fatalf("selected = %d", m.list.Index())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")})
	if got := m.period.Start.Format("2006-01-02"); got != "2026-08-08" {
		t.Fatalf("period start = %s", got)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	if m.period.Kind != storage.Week {
		t.Fatalf("period = %s", m.period.Kind)
	}
	if !strings.Contains(m.View(), "03 Aug 2026") {
		t.Fatalf("view = %q", m.View())
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := m.list.SelectedItem().(statsTaskItem); !ok || !m.detail {
		t.Fatalf("detail list item = %#v", m.list.SelectedItem())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if _, ok := m.list.SelectedItem().(statsProjectItem); !ok || m.detail {
		t.Fatalf("project list item = %#v", m.list.SelectedItem())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if m.GlobalKeysEnabled() {
		t.Fatal("global keys remain enabled while filtering")
	}
}

func TestStatsCurrentPeriodAdvancesAtCalendarBoundary(t *testing.T) {
	tests := []struct {
		name      string
		kind      storage.PeriodKind
		before    time.Time
		after     time.Time
		wantStart string
	}{
		{
			name: "day", kind: storage.Day,
			before:    time.Date(2026, 8, 9, 23, 59, 59, 0, time.UTC),
			after:     time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-08-10",
		},
		{
			name: "week", kind: storage.Week,
			before:    time.Date(2026, 8, 9, 23, 59, 59, 0, time.UTC),
			after:     time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-08-10",
		},
		{
			name: "month", kind: storage.Month,
			before:    time.Date(2026, 8, 31, 23, 59, 59, 0, time.UTC),
			after:     time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-09-01",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			period, err := storage.CurrentPeriod(test.kind, test.before)
			if err != nil {
				t.Fatal(err)
			}
			m := NewStats(t.Context(), nil)
			m.now, m.period = test.before, period
			m, _ = m.Update(statsTickMsg(test.after))
			if got := m.period.Start.Format("2006-01-02"); got != test.wantStart {
				t.Fatalf("period start = %s, want %s", got, test.wantStart)
			}
		})
	}
}

func TestStatsHistoricalPeriodDoesNotAdvanceAtCalendarBoundary(t *testing.T) {
	before := time.Date(2026, 8, 9, 23, 59, 59, 0, time.UTC)
	period, err := storage.CurrentPeriod(storage.Day, before)
	if err != nil {
		t.Fatal(err)
	}
	period = storage.MovePeriod(period, -1)
	m := NewStats(t.Context(), nil)
	m.now, m.period = before, period

	m, _ = m.Update(statsTickMsg(time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)))

	if got := m.period.Start.Format("2006-01-02"); got != "2026-08-08" {
		t.Fatalf("period start = %s, want 2026-08-08", got)
	}
}

func TestStatsAppliesFilterResultsAfterRefresh(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	period, err := storage.CurrentPeriod(storage.Day, now)
	if err != nil {
		t.Fatal(err)
	}
	project1, project2, taskID, rateID := 1, 2, 1, 1
	m := NewStats(t.Context(), nil)
	m.now, m.period, m.loading = now, period, false
	m.projects = []storage.Project{
		{ID: 1, Name: "Acme", RateID: 1},
		{ID: 2, Name: "Beta", RateID: 1},
	}
	m.tasks = []storage.Task{{ID: 1, Name: "Build", ProjectID: 1}}
	m.rates = []storage.Rate{{ID: 1, AmountMinor: 10_000, Currency: "USD"}}
	m.entries = []storage.Entry{
		{TaskID: &taskID, ProjectID: &project1, RateID: &rateID, StartedAt: now.Add(-time.Hour)},
		{TaskID: &taskID, ProjectID: &project2, RateID: &rateID, StartedAt: now.Add(-time.Hour)},
	}
	m.refresh()
	m.list.SetFilterText("Beta")

	cmd := m.refresh()
	if cmd == nil {
		t.Fatal("filtered refresh returned no command")
	}
	if got := len(m.list.VisibleItems()); got != 0 {
		t.Fatalf("visible items before filter result = %d, want 0", got)
	}
	m, _ = m.Update(cmd())

	items := m.list.VisibleItems()
	if len(items) != 1 || items[0].FilterValue() != "Beta" {
		t.Fatalf("visible items = %#v, want Beta", items)
	}
}
