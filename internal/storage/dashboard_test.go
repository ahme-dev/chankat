package storage_test

import (
	"testing"
	"time"

	"chankat/internal/storage"
)

func TestSummarizeDashboardClipsEntriesAndGroupsProjects(t *testing.T) {
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour)
	now := end.Add(-12 * time.Hour)
	rate := storage.Rate{ID: 1, AmountMinor: 10_000, Currency: "USD"}
	projects := []storage.Project{
		{ID: 1, Name: "Acme", RateID: 1},
		{ID: 2, Name: "Beta", RateID: 1},
	}
	tasks := []storage.Task{
		{ID: 1, Name: "Build", ProjectID: 1},
		{ID: 2, Name: "Review", ProjectID: 2},
	}
	project1, project2, task1, task2, rateID := 1, 2, 1, 2, 1
	beforeEnd := start.Add(time.Hour)
	activeStart := now.Add(-2 * time.Hour)
	entries := []storage.Entry{
		{
			TaskID: &task1, ProjectID: &project1, RateID: &rateID,
			StartedAt: start.Add(-time.Hour), EndedAt: &beforeEnd,
		},
		{
			TaskID: &task2, ProjectID: &project2, RateID: &rateID,
			StartedAt: activeStart,
		},
		{
			TaskID: &task1, ProjectID: &project1, RateID: &rateID,
			StartedAt: end.Add(time.Hour),
		},
	}
	payments := []storage.Payment{
		{ProjectID: 1, AmountMinor: 5_000, Currency: "USD", PaidForDate: start},
		{ProjectID: 2, AmountMinor: 9_000, Currency: "USD", PaidForDate: end},
	}

	got := storage.SummarizeDashboard(
		projects, tasks, []storage.Rate{rate}, entries, payments,
		start, end, now,
	)

	if got.Tracked != 3*time.Hour {
		t.Fatalf("tracked = %s, want 3h", got.Tracked)
	}
	if got.EarnedMinor["USD"] != 30_000 ||
		got.PaidMinor["USD"] != 5_000 || got.NetMinor["USD"] != 25_000 {
		t.Fatalf("amounts = earned %v paid %v net %v",
			got.EarnedMinor, got.PaidMinor, got.NetMinor)
	}
	if len(got.Projects) != 2 || got.Projects[0].ProjectName != "Beta" {
		t.Fatalf("projects = %#v", got.Projects)
	}
	if len(got.Projects[0].Tasks) != 1 ||
		got.Projects[0].Tasks[0].TaskName != "Review" {
		t.Fatalf("tasks = %#v", got.Projects[0].Tasks)
	}
}

func TestSummarizeDashboardKeepsCurrenciesSeparate(t *testing.T) {
	start := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	projectID, taskID, usdRateID, eurRateID := 1, 1, 1, 2
	firstEnd := start.Add(time.Hour)
	secondEnd := firstEnd.Add(time.Hour)
	got := storage.SummarizeDashboard(
		[]storage.Project{{ID: 1, Name: "Acme", RateID: 1}},
		[]storage.Task{{ID: 1, Name: "Build", ProjectID: 1}},
		[]storage.Rate{
			{ID: 1, AmountMinor: 10_000, Currency: "USD"},
			{ID: 2, AmountMinor: 8_000, Currency: "EUR"},
		},
		[]storage.Entry{
			{TaskID: &taskID, ProjectID: &projectID, RateID: &usdRateID,
				StartedAt: start, EndedAt: &firstEnd},
			{TaskID: &taskID, ProjectID: &projectID, RateID: &eurRateID,
				StartedAt: firstEnd, EndedAt: &secondEnd},
		},
		nil, start, end, end,
	)

	if got.EarnedMinor["USD"] != 10_000 || got.EarnedMinor["EUR"] != 8_000 {
		t.Fatalf("earned = %v", got.EarnedMinor)
	}
}

func TestSummarizeDashboardTimelineSplitsEntriesAcrossHours(t *testing.T) {
	start := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	period, err := storage.CurrentPeriod(storage.Day, start.Add(12*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	firstEnd := start.Add(2*time.Hour + 30*time.Minute)
	entries := []storage.Entry{
		{StartedAt: start.Add(30 * time.Minute), EndedAt: &firstEnd},
		{StartedAt: start.Add(3 * time.Hour)},
	}

	buckets := storage.SummarizeDashboardTimeline(
		entries, period, start.Add(4*time.Hour),
	)

	if len(buckets) != 24 {
		t.Fatalf("bucket count = %d, want 24", len(buckets))
	}
	want := []time.Duration{30 * time.Minute, time.Hour, 30 * time.Minute, time.Hour}
	for i, duration := range want {
		if buckets[i].Tracked != duration {
			t.Errorf("bucket %d = %s, want %s", i, buckets[i].Tracked, duration)
		}
	}
}
