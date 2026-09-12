package storage_test

import (
	"testing"
	"time"

	"chankat/internal/storage"
)

func TestSummaries(t *testing.T) {
	rate := storage.Rate{
		ID: 1, Name: "Standard", AmountMinor: 10_000, Currency: "USD",
	}
	project := storage.Project{ID: 1, Name: "Acme", RateID: rate.ID}
	task := storage.Task{ID: 1, Name: "Build", ProjectID: project.ID}
	taskID, projectID, rateID := task.ID, project.ID, rate.ID
	startedAt := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	firstEnd := startedAt.Add(90 * time.Minute)
	now := startedAt.Add(2 * time.Hour)
	entries := []storage.Entry{
		{
			ID: 1, TaskID: &taskID, ProjectID: &projectID, RateID: &rateID,
			StartedAt: startedAt, EndedAt: &firstEnd,
		},
		{
			ID: 2, TaskID: &taskID, ProjectID: &projectID, RateID: &rateID,
			StartedAt: firstEnd,
		},
	}
	payments := []storage.Payment{{
		ProjectID: project.ID, AmountMinor: 5_000, Currency: "USD",
	}}

	rates := storage.SummarizeRates(
		[]storage.Rate{rate}, []storage.Project{project},
	)
	if rates[0].ProjectCount != 1 {
		t.Fatalf("got rate summary %#v", rates[0])
	}

	projects := storage.SummarizeProjects(
		[]storage.Project{project}, []storage.Rate{rate}, entries, payments, now,
	)
	if projects[0].Tracked != 2*time.Hour ||
		projects[0].BalanceMinor["USD"] != 15_000 {
		t.Fatalf("got project summary %#v", projects[0])
	}

	tasks := storage.SummarizeTasks(
		[]storage.Task{task},
		[]storage.Project{project},
		entries,
		[]storage.Rate{rate},
		now,
	)
	if !tasks[0].Active ||
		tasks[0].Tracked != 2*time.Hour ||
		tasks[0].EarnedMinor["USD"] != 20_000 ||
		tasks[0].LastEntryID != 1 {
		t.Fatalf("got task summary %#v", tasks[0])
	}
}

func TestProjectBalanceCarriesPrepaymentForward(t *testing.T) {
	projectID, rateID := 1, 1
	startedAt := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(time.Hour)
	paidAt := endedAt.AddDate(0, 0, 30)
	summaries := storage.SummarizeProjects(
		[]storage.Project{{ID: projectID, RateID: rateID}},
		[]storage.Rate{{ID: rateID, AmountMinor: 10_000, Currency: "USD"}},
		[]storage.Entry{{
			ProjectID: &projectID, RateID: &rateID,
			StartedAt: startedAt, EndedAt: &endedAt,
		}},
		[]storage.Payment{{
			ProjectID: projectID, AmountMinor: 15_000,
			Currency: "USD", PaidAt: paidAt,
		}},
		paidAt,
	)
	if got := summaries[0].BalanceMinor["USD"]; got != -5_000 {
		t.Fatalf("balance = %d, want 5000 credit", got)
	}
}
