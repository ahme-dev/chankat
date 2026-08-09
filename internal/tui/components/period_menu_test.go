package components

import (
	"testing"
	"time"

	"chankat/internal/storage"
)

func TestPeriodMenuSkipsDatesUnlessCustom(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	menu := NewPeriodMenu(storage.Period{Kind: storage.All}, now, 80)
	menu.form.NextGroup()
	if !menu.Completed() {
		t.Fatal("all-time period did not skip the date range")
	}

	menu = NewPeriodMenu(storage.Period{Kind: storage.All}, now, 80)
	menu.kind = customPeriod
	menu.form.NextGroup()
	if menu.Completed() {
		t.Fatal("custom period skipped the date range")
	}
}

func TestPeriodMenuReturnsCustomPeriod(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	menu := NewPeriodMenu(storage.Period{Kind: storage.All}, now, 80)
	menu.kind = customPeriod
	menu.from = "2026-08-01"
	menu.to = "2026-08-03"

	period, err := menu.Period(now)
	if err != nil {
		t.Fatal(err)
	}
	if got := period.Start.Format("2006-01-02"); got != "2026-08-01" {
		t.Fatalf("period start = %s", got)
	}
	if got := period.End.Format("2006-01-02"); got != "2026-08-04" {
		t.Fatalf("period end = %s", got)
	}
}
