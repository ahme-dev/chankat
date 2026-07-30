package storage_test

import (
	"strings"
	"testing"
	"time"

	"chankat/internal/storage"
)

func TestCreateEntry(t *testing.T) {
	t.Run("persists optional snapshots and times", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		rate := fixtureRate(t, stor)
		project := fixtureProject(t, stor)
		if err := stor.CreateTask(ctx, storage.Task{
			Name: "task", ProjectID: project.ID,
		}); err != nil {
			t.Fatal(err)
		}
		task, err := stor.GetTask(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		started := time.Unix(1_700_000_000, 0)
		ended := started.Add(time.Hour)

		if err := stor.CreateEntry(ctx, storage.Entry{
			TaskID: &task.ID, ProjectID: &project.ID, RateID: &rate.ID,
			StartedAt: started, EndedAt: &ended, Note: "work",
		}); err != nil {
			t.Fatal(err)
		}
		entry, err := stor.GetEntry(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if entry.TaskID == nil || *entry.TaskID != task.ID ||
			entry.ProjectID == nil || *entry.ProjectID != project.ID ||
			entry.RateID == nil || *entry.RateID != rate.ID ||
			!entry.StartedAt.Equal(started) ||
			entry.EndedAt == nil || !entry.EndedAt.Equal(ended) ||
			entry.Note != "work" {
			t.Fatalf("unexpected entry: %#v", entry)
		}
	})

	t.Run("rejects missing start and reversed range", func(t *testing.T) {
		stor := fixtureStorage(t)
		if err := stor.CreateEntry(t.Context(), storage.Entry{}); err == nil {
			t.Fatal("entry without a start time succeeded")
		}
		startedAt := time.Unix(1_700_000_000, 0)
		endedAt := startedAt.Add(-time.Minute)
		if err := stor.CreateEntry(t.Context(), storage.Entry{
			StartedAt: startedAt, EndedAt: &endedAt,
		}); err == nil {
			t.Fatal("entry with reversed times succeeded")
		}
	})
}

func TestGetEntries(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		stor := fixtureStorage(t)
		entries, err := stor.GetEntries(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("got %d entries, want 0", len(entries))
		}
	})
}

func TestGetActiveEntries(t *testing.T) {
	t.Run("allows multiple active entries", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		for i := 0; i < 2; i++ {
			if err := stor.CreateEntry(ctx, storage.Entry{
				StartedAt: time.Unix(int64(1_700_000_000+i), 0),
			}); err != nil {
				t.Fatal(err)
			}
		}
		ended := time.Unix(1_700_000_100, 0)
		if err := stor.CreateEntry(ctx, storage.Entry{
			StartedAt: time.Unix(1_700_000_050, 0), EndedAt: &ended,
		}); err != nil {
			t.Fatal(err)
		}

		entries, err := stor.GetActiveEntries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 {
			t.Fatalf("got %d active entries, want 2", len(entries))
		}
	})
}

func TestGetEntry(t *testing.T) {
	t.Run("missing entry", func(t *testing.T) {
		stor := fixtureStorage(t)
		_, err := stor.GetEntry(t.Context(), 999)
		if err == nil || !strings.Contains(err.Error(), "entry 999 not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestUpdateEntry(t *testing.T) {
	t.Run("closes active entry", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		started := time.Unix(1_700_000_000, 0)
		if err := stor.CreateEntry(ctx, storage.Entry{StartedAt: started}); err != nil {
			t.Fatal(err)
		}
		ended := started.Add(time.Hour)
		if err := stor.UpdateEntry(ctx, storage.Entry{
			ID: 1, StartedAt: started, EndedAt: &ended,
		}); err != nil {
			t.Fatal(err)
		}
		active, err := stor.GetActiveEntries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(active) != 0 {
			t.Fatalf("got %d active entries, want 0", len(active))
		}
	})
}

func TestPauseEntry(t *testing.T) {
	t.Run("closes active entry", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		startedAt := time.Unix(1_700_000_000, 0)
		endedAt := startedAt.Add(time.Hour)
		if err := stor.CreateEntry(ctx, storage.Entry{StartedAt: startedAt}); err != nil {
			t.Fatal(err)
		}

		if err := stor.PauseEntry(ctx, 1, endedAt); err != nil {
			t.Fatal(err)
		}
		entry, err := stor.GetEntry(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if entry.EndedAt == nil || !entry.EndedAt.Equal(endedAt) {
			t.Fatalf("unexpected end time: %v", entry.EndedAt)
		}
	})

	t.Run("rejects completed entry", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		startedAt := time.Unix(1_700_000_000, 0)
		endedAt := startedAt.Add(time.Hour)
		if err := stor.CreateEntry(ctx, storage.Entry{
			StartedAt: startedAt, EndedAt: &endedAt,
		}); err != nil {
			t.Fatal(err)
		}

		err := stor.PauseEntry(ctx, 1, endedAt.Add(time.Hour))
		if err == nil || !strings.Contains(err.Error(), "not active") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects an end before the start", func(t *testing.T) {
		stor := fixtureStorage(t)
		startedAt := time.Unix(1_700_000_000, 0)
		if err := stor.CreateEntry(t.Context(), storage.Entry{
			StartedAt: startedAt,
		}); err != nil {
			t.Fatal(err)
		}

		if err := stor.PauseEntry(
			t.Context(), 1, startedAt.Add(-time.Second),
		); err == nil {
			t.Fatal("pause before start succeeded")
		}
		entry, err := stor.GetEntry(t.Context(), 1)
		if err != nil {
			t.Fatal(err)
		}
		if entry.EndedAt != nil {
			t.Fatalf("invalid pause changed entry: %#v", entry)
		}
	})
}

func TestDeleteEntry(t *testing.T) {
	t.Run("existing entry", func(t *testing.T) {
		stor := fixtureStorage(t)
		if err := stor.CreateEntry(t.Context(), storage.Entry{StartedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
		if err := stor.DeleteEntry(t.Context(), 1); err != nil {
			t.Fatal(err)
		}
	})
}

func TestAssignEntriesToProject(t *testing.T) {
	t.Run("assigns and clears projects", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		from := fixtureProject(t, stor)
		to := fixtureProject(t, stor)

		for i := 0; i < 2; i++ {
			if err := stor.CreateEntry(ctx, storage.Entry{
				ProjectID: &from.ID,
				StartedAt: time.Unix(int64(1_700_000_000+i), 0),
			}); err != nil {
				t.Fatal(err)
			}
		}

		if err := stor.AssignEntriesToProject(ctx, []int{1, 2}, &to.ID); err != nil {
			t.Fatal(err)
		}
		entries, err := stor.GetEntries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.ProjectID == nil || *entry.ProjectID != to.ID {
				t.Errorf("entry %d has project %v, want %d", entry.ID, entry.ProjectID, to.ID)
			}
		}

		if err := stor.AssignEntriesToProject(ctx, []int{1, 2}, nil); err != nil {
			t.Fatal(err)
		}
		entries, err = stor.GetEntries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.ProjectID != nil {
				t.Errorf("entry %d still has a project", entry.ID)
			}
		}
	})
}

func TestAssignEntriesToRate(t *testing.T) {
	t.Run("assigns and clears rates", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		from := fixtureRate(t, stor)
		to := fixtureRate(t, stor)

		for i := 0; i < 2; i++ {
			if err := stor.CreateEntry(ctx, storage.Entry{
				RateID:    &from.ID,
				StartedAt: time.Unix(int64(1_700_000_000+i), 0),
			}); err != nil {
				t.Fatal(err)
			}
		}

		if err := stor.AssignEntriesToRate(ctx, []int{1, 2}, &to.ID); err != nil {
			t.Fatal(err)
		}
		entries, err := stor.GetEntries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.RateID == nil || *entry.RateID != to.ID {
				t.Errorf("entry %d has rate %v, want %d", entry.ID, entry.RateID, to.ID)
			}
		}

		if err := stor.AssignEntriesToRate(ctx, []int{1, 2}, nil); err != nil {
			t.Fatal(err)
		}
		entries, err = stor.GetEntries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.RateID != nil {
				t.Errorf("entry %d still has a rate", entry.ID)
			}
		}
	})

	t.Run("rejects an empty selection", func(t *testing.T) {
		stor := fixtureStorage(t)
		err := stor.AssignEntriesToRate(t.Context(), nil, nil)
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}
