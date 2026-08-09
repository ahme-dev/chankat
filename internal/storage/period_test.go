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
