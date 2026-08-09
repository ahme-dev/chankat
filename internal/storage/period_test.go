package storage

import (
	"testing"
	"time"
)

func TestCurrentWeekStartsMonday(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	period, err := CurrentPeriod(Week, now)
	if err != nil {
		t.Fatal(err)
	}
	if got := period.Start.Format("2006-01-02"); got != "2026-08-03" {
		t.Fatalf("start = %s", got)
	}
	if got := period.End.Format("2006-01-02"); got != "2026-08-10" {
		t.Fatalf("end = %s", got)
	}
}

func TestMoveMonthUsesCalendarMonths(t *testing.T) {
	now := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	period, err := CurrentPeriod(Month, now)
	if err != nil {
		t.Fatal(err)
	}
	period = MovePeriod(period, -1)
	if got := period.Label(); got != "01 Feb 2026 – 28 Feb 2026" {
		t.Fatalf("label = %q", got)
	}
}

func TestDayUsesLocalCalendarAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, location)
	period, err := CurrentPeriod(Day, now)
	if err != nil {
		t.Fatal(err)
	}
	if got := period.End.Sub(period.Start); got != 23*time.Hour {
		t.Fatalf("duration = %s, want 23h", got)
	}
}

func TestEntriesInPeriodClipsBoundariesAndActiveEntries(t *testing.T) {
	start := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	now := start.Add(12 * time.Hour)
	period, err := CurrentPeriod(Day, now)
	if err != nil {
		t.Fatal(err)
	}
	endedAt := start.Add(time.Hour)
	entries := []Entry{
		{ID: 1, StartedAt: start.Add(-time.Hour), EndedAt: &endedAt},
		{ID: 2, StartedAt: start.Add(10 * time.Hour)},
		{ID: 3, StartedAt: period.End.Add(time.Hour)},
	}

	got := EntriesInPeriod(entries, period, now)

	if len(got) != 2 {
		t.Fatalf("entries = %#v", got)
	}
	if !got[0].StartedAt.Equal(start) || got[0].EndedAt == nil ||
		!got[0].EndedAt.Equal(endedAt) {
		t.Fatalf("clipped entry = %#v", got[0])
	}
	if got[1].EndedAt != nil {
		t.Fatalf("active entry became completed: %#v", got[1])
	}
}

func TestStepPeriodKind(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	period, err := CurrentPeriod(Day, now)
	if err != nil {
		t.Fatal(err)
	}

	period = StepPeriodKind(period, 1, now)
	if period.Kind != Week || period.Start.Day() != 3 {
		t.Fatalf("day to week = %#v", period)
	}
	period = StepPeriodKind(period, 1, now)
	if period.Kind != Month || period.Start.Day() != 1 {
		t.Fatalf("week to month = %#v", period)
	}
	period = StepPeriodKind(period, 1, now)
	if period.Kind != All {
		t.Fatalf("month to all = %#v", period)
	}
	period = StepPeriodKind(period, 1, now)
	if period.Kind != All {
		t.Fatalf("all did not remain at the upper bound: %#v", period)
	}
	period, err = CurrentPeriod(Day, now)
	if err != nil {
		t.Fatal(err)
	}
	period = StepPeriodKind(period, -1, now)
	if period.Kind != Day {
		t.Fatalf("day did not remain at the lower bound: %#v", period)
	}
}

func TestStepCustomPeriodReturnsToPreset(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	custom, err := CustomPeriod(now.AddDate(0, 0, -3), now)
	if err != nil {
		t.Fatal(err)
	}
	if got := StepPeriodKind(custom, 1, now).Kind; got != All {
		t.Fatalf("custom down = %s, want all", got)
	}
	if got := StepPeriodKind(custom, -1, now).Kind; got != Month {
		t.Fatalf("custom up = %s, want month", got)
	}
}
