package storage_test

import (
	"strings"
	"testing"
	"time"

	"chankat/internal/storage"
)

func TestCreateTask(t *testing.T) {
	t.Run("requires project", func(t *testing.T) {
		stor := fixtureStorage(t)
		if err := stor.CreateTask(t.Context(), storage.Task{Name: "independent"}); err != nil {
			return
		}
		t.Fatal("expected a foreign-key error")
	})

	t.Run("with project", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		if err := stor.CreateTask(t.Context(), storage.Task{
			Name: "project task", ProjectID: project.ID,
		}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCreateTaskAndStart(t *testing.T) {
	t.Run("creates task and active entry", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		project := fixtureProject(t, stor)
		startedAt := time.Unix(1_700_000_000, 0)

		if err := stor.CreateTaskAndStart(ctx, storage.Task{
			Name: "tracked task", ProjectID: project.ID,
		}, startedAt); err != nil {
			t.Fatal(err)
		}

		tasks, err := stor.GetTasks(ctx)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := stor.GetActiveEntries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 1 || len(entries) != 1 {
			t.Fatalf("got %d tasks and %d entries", len(tasks), len(entries))
		}
		if entries[0].TaskID == nil || *entries[0].TaskID != tasks[0].ID ||
			entries[0].ProjectID == nil || *entries[0].ProjectID != project.ID ||
			entries[0].RateID == nil || *entries[0].RateID != project.RateID ||
			!entries[0].StartedAt.Equal(startedAt) {
			t.Fatalf("unexpected entry: %#v", entries[0])
		}
	})

	t.Run("uses task rate override", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		project := fixtureProject(t, stor)
		if err := stor.CreateRate(ctx, storage.Rate{
			Name: "override", AmountMinor: 20_000, Currency: "USD",
		}); err != nil {
			t.Fatal(err)
		}
		rates, err := stor.GetRates(ctx)
		if err != nil {
			t.Fatal(err)
		}
		overrideRateID := rates[1].ID
		if err := stor.CreateTaskAndStart(ctx, storage.Task{
			Name: "custom", ProjectID: project.ID, RateID: &overrideRateID,
		}, time.Unix(1_700_000_000, 0)); err != nil {
			t.Fatal(err)
		}
		entries, err := stor.GetEntries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].RateID == nil ||
			*entries[0].RateID != overrideRateID {
			t.Fatalf("override was not captured: %#v", entries)
		}
	})

	t.Run("rolls back for missing project", func(t *testing.T) {
		stor := fixtureStorage(t)
		err := stor.CreateTaskAndStart(t.Context(), storage.Task{
			Name: "invalid", ProjectID: 999,
		}, time.Now())
		if err == nil {
			t.Fatal("expected an error")
		}
		tasks, getErr := stor.GetTasks(t.Context())
		if getErr != nil {
			t.Fatal(getErr)
		}
		if len(tasks) != 0 {
			t.Fatalf("got %d tasks, want 0", len(tasks))
		}
	})
}

func TestStartTask(t *testing.T) {
	stor := fixtureStorage(t)
	ctx := t.Context()
	project := fixtureProject(t, stor)
	if err := stor.CreateTask(ctx, storage.Task{
		Name: "tracked task", ProjectID: project.ID,
	}); err != nil {
		t.Fatal(err)
	}
	startedAt := time.Unix(1_700_000_000, 0)
	if err := stor.StartTask(ctx, 1, startedAt); err != nil {
		t.Fatal(err)
	}
	entries, err := stor.GetActiveEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 ||
		entries[0].TaskID == nil || *entries[0].TaskID != 1 ||
		entries[0].ProjectID == nil || *entries[0].ProjectID != project.ID ||
		entries[0].RateID == nil || *entries[0].RateID != project.RateID {
		t.Fatalf("unexpected active entries: %#v", entries)
	}

	if err := stor.StartTask(ctx, 999, startedAt); err == nil {
		t.Fatal("missing task started")
	}
	entries, err = stor.GetEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("failed start created an entry: %#v", entries)
	}
}

func TestTaskEntriesCaptureProjectRateChanges(t *testing.T) {
	stor := fixtureStorage(t)
	ctx := t.Context()
	project := fixtureProject(t, stor)
	if err := stor.CreateTask(ctx, storage.Task{
		Name: "tracked task", ProjectID: project.ID,
	}); err != nil {
		t.Fatal(err)
	}
	startedAt := time.Unix(1_700_000_000, 0)
	if err := stor.StartTask(ctx, 1, startedAt); err != nil {
		t.Fatal(err)
	}
	if err := stor.PauseTask(ctx, 1, startedAt.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	if err := stor.CreateRate(ctx, storage.Rate{
		Name: "increased", AmountMinor: 10_000, Currency: "USD",
	}); err != nil {
		t.Fatal(err)
	}
	rates, err := stor.GetRates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	newRateID := rates[len(rates)-1].ID
	project.RateID = newRateID
	if err := stor.UpdateProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := stor.StartTask(ctx, 1, startedAt.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := stor.PauseTask(ctx, 1, startedAt.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}

	entries, err := stor.GetEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].RateID == nil ||
		*entries[0].RateID != rates[0].ID || entries[1].RateID == nil ||
		*entries[1].RateID != newRateID {
		t.Fatalf("entries did not preserve rate history: %#v", entries)
	}
	summaries := storage.SummarizeProjects(
		[]storage.Project{project}, rates, entries, nil,
		startedAt.Add(3*time.Hour),
	)
	if got := summaries[0].BalanceMinor["USD"]; got != 17_500 {
		t.Fatalf("historical-rate earnings = %d, want 17500", got)
	}
}

func TestTaskRateOverrideAndProjectChangesAffectFutureEntries(t *testing.T) {
	stor := fixtureStorage(t)
	ctx := t.Context()
	firstProject := fixtureProject(t, stor)
	if err := stor.CreateRate(ctx, storage.Rate{
		Name: "second project", AmountMinor: 20_000, Currency: "USD",
	}); err != nil {
		t.Fatal(err)
	}
	if err := stor.CreateRate(ctx, storage.Rate{
		Name: "task override", AmountMinor: 30_000, Currency: "USD",
	}); err != nil {
		t.Fatal(err)
	}
	rates, err := stor.GetRates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := stor.CreateProject(ctx, storage.Project{
		Name: "second", RateID: rates[1].ID,
	}); err != nil {
		t.Fatal(err)
	}
	projects, err := stor.GetProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := stor.CreateTask(ctx, storage.Task{
		Name: "movable", ProjectID: firstProject.ID,
	}); err != nil {
		t.Fatal(err)
	}

	startedAt := time.Unix(1_700_000_000, 0)
	startAndStop := func(offset time.Duration) {
		t.Helper()
		start := startedAt.Add(offset)
		if err := stor.StartTask(ctx, 1, start); err != nil {
			t.Fatal(err)
		}
		if err := stor.PauseTask(ctx, 1, start.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	startAndStop(0)

	overrideRateID := rates[2].ID
	if err := stor.UpdateTask(ctx, storage.Task{
		ID: 1, Name: "movable", ProjectID: projects[1].ID,
		RateID: &overrideRateID,
	}); err != nil {
		t.Fatal(err)
	}
	startAndStop(2 * time.Hour)

	if err := stor.UpdateTask(ctx, storage.Task{
		ID: 1, Name: "movable", ProjectID: projects[1].ID,
	}); err != nil {
		t.Fatal(err)
	}
	startAndStop(4 * time.Hour)

	task, err := stor.GetTask(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if task.ProjectID != projects[1].ID || task.RateID != nil {
		t.Fatalf("unexpected updated task: %#v", task)
	}
	entries, err := stor.GetEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantProjects := []int{firstProject.ID, projects[1].ID, projects[1].ID}
	wantRates := []int{firstProject.RateID, overrideRateID, rates[1].ID}
	if len(entries) != len(wantRates) {
		t.Fatalf("got %d entries, want %d", len(entries), len(wantRates))
	}
	for i, entry := range entries {
		if entry.ProjectID == nil || *entry.ProjectID != wantProjects[i] ||
			entry.RateID == nil || *entry.RateID != wantRates[i] {
			t.Fatalf("entry %d = %#v", i, entry)
		}
	}
}

func TestPauseTask(t *testing.T) {
	stor := fixtureStorage(t)
	ctx := t.Context()
	project := fixtureProject(t, stor)
	if err := stor.CreateTask(ctx, storage.Task{
		Name: "tracked task", ProjectID: project.ID,
	}); err != nil {
		t.Fatal(err)
	}
	startedAt := time.Unix(1_700_000_000, 0)
	for range 2 {
		if err := stor.StartTask(ctx, 1, startedAt); err != nil {
			t.Fatal(err)
		}
	}
	endedAt := startedAt.Add(time.Hour)
	if err := stor.PauseTask(ctx, 1, endedAt); err != nil {
		t.Fatal(err)
	}
	active, err := stor.GetActiveEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("%d entries remain active", len(active))
	}
	if err := stor.PauseTask(ctx, 1, endedAt); err == nil {
		t.Fatal("inactive task stopped")
	}
}

func TestCreateTaskAndEntry(t *testing.T) {
	stor := fixtureStorage(t)
	ctx := t.Context()
	project := fixtureProject(t, stor)
	startedAt := time.Unix(1_700_000_000, 0)
	endedAt := startedAt.Add(90 * time.Minute)

	if err := stor.CreateTaskAndEntry(
		ctx,
		storage.Task{Name: "past task", ProjectID: project.ID},
		storage.Entry{
			StartedAt: startedAt,
			EndedAt:   &endedAt,
			Note:      "missed entry",
		},
	); err != nil {
		t.Fatal(err)
	}

	tasks, err := stor.GetTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := stor.GetEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || len(entries) != 1 {
		t.Fatalf("got %d tasks and %d entries", len(tasks), len(entries))
	}
	entry := entries[0]
	if entry.TaskID == nil || *entry.TaskID != tasks[0].ID ||
		entry.ProjectID == nil || *entry.ProjectID != project.ID ||
		entry.RateID == nil || *entry.RateID != project.RateID ||
		!entry.StartedAt.Equal(startedAt) ||
		entry.EndedAt == nil || !entry.EndedAt.Equal(endedAt) ||
		entry.Note != "missed entry" {
		t.Fatalf("unexpected entry: %#v", entry)
	}
}

func TestCreateTaskAndEntryRejectsInvalidRange(t *testing.T) {
	stor := fixtureStorage(t)
	project := fixtureProject(t, stor)
	startedAt := time.Unix(1_700_000_000, 0)
	endedAt := startedAt.Add(-time.Minute)

	err := stor.CreateTaskAndEntry(
		t.Context(),
		storage.Task{Name: "invalid", ProjectID: project.ID},
		storage.Entry{StartedAt: startedAt, EndedAt: &endedAt},
	)
	if err == nil {
		t.Fatal("task with reversed entry times succeeded")
	}
	tasks, getErr := stor.GetTasks(t.Context())
	if getErr != nil {
		t.Fatal(getErr)
	}
	if len(tasks) != 0 {
		t.Fatalf("invalid entry left %d tasks behind", len(tasks))
	}
}

func TestGetTasks(t *testing.T) {
	t.Run("ordered by ID", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		project := fixtureProject(t, stor)
		for _, name := range []string{"first", "second"} {
			if err := stor.CreateTask(ctx, storage.Task{
				Name: name, ProjectID: project.ID,
			}); err != nil {
				t.Fatal(err)
			}
		}
		tasks, err := stor.GetTasks(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 2 || tasks[0].Name != "first" || tasks[1].Name != "second" {
			t.Fatalf("unexpected tasks: %#v", tasks)
		}
	})
}

func TestGetTask(t *testing.T) {
	t.Run("existing task", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		if err := stor.CreateTask(t.Context(), storage.Task{
			Name: "task", ProjectID: project.ID,
		}); err != nil {
			t.Fatal(err)
		}
		task, err := stor.GetTask(t.Context(), 1)
		if err != nil {
			t.Fatal(err)
		}
		if task.Name != "task" {
			t.Fatalf("got task %#v", task)
		}
	})

	t.Run("preserves time entries", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		if err := stor.CreateTask(t.Context(), storage.Task{
			Name: "task", ProjectID: project.ID,
		}); err != nil {
			t.Fatal(err)
		}
		startedAt := time.Unix(1_700_000_000, 0)
		endedAt := startedAt.Add(time.Hour)
		if err := stor.CreateEntryForTask(
			t.Context(), 1, startedAt, &endedAt, "work",
		); err != nil {
			t.Fatal(err)
		}
		if err := stor.DeleteTask(t.Context(), 1); err != nil {
			t.Fatal(err)
		}
		entries, err := stor.GetEntries(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].TaskID != nil ||
			entries[0].ProjectID == nil || *entries[0].ProjectID != project.ID ||
			entries[0].RateID == nil || *entries[0].RateID != project.RateID {
			t.Fatalf("task deletion lost accounting history: %#v", entries)
		}
	})

	t.Run("missing task", func(t *testing.T) {
		stor := fixtureStorage(t)
		_, err := stor.GetTask(t.Context(), 999)
		if err == nil || !strings.Contains(err.Error(), "task 999 not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestUpdateTask(t *testing.T) {
	t.Run("updates existing task", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		if err := stor.CreateTask(t.Context(), storage.Task{
			Name: "old", ProjectID: project.ID,
		}); err != nil {
			t.Fatal(err)
		}
		if err := stor.UpdateTask(t.Context(), storage.Task{
			ID: 1, Name: "new", ProjectID: project.ID,
		}); err != nil {
			t.Fatal(err)
		}
		task, err := stor.GetTask(t.Context(), 1)
		if err != nil {
			t.Fatal(err)
		}
		if task.Name != "new" {
			t.Fatalf("got task %#v", task)
		}
	})
}

func TestDeleteTask(t *testing.T) {
	t.Run("existing task", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		if err := stor.CreateTask(t.Context(), storage.Task{
			Name: "task", ProjectID: project.ID,
		}); err != nil {
			t.Fatal(err)
		}
		if err := stor.DeleteTask(t.Context(), 1); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing task", func(t *testing.T) {
		stor := fixtureStorage(t)
		err := stor.DeleteTask(t.Context(), 999)
		if err == nil || !strings.Contains(err.Error(), "task 999 not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestAssignTasksToProject(t *testing.T) {
	t.Run("assigns multiple tasks", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		from := fixtureProject(t, stor)
		to := fixtureProject(t, stor)

		for _, name := range []string{"first", "second"} {
			if err := stor.CreateTask(ctx, storage.Task{
				Name: name, ProjectID: from.ID,
			}); err != nil {
				t.Fatal(err)
			}
		}

		if err := stor.AssignTasksToProject(ctx, []int{1, 2}, to.ID); err != nil {
			t.Fatal(err)
		}
		tasks, err := stor.GetTasks(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, task := range tasks {
			if task.ProjectID != to.ID {
				t.Errorf("task %d has project %d, want %d", task.ID, task.ProjectID, to.ID)
			}
		}
	})

	t.Run("is atomic when a task is missing", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		from := fixtureProject(t, stor)
		to := fixtureProject(t, stor)
		if err := stor.CreateTask(ctx, storage.Task{
			Name: "task", ProjectID: from.ID,
		}); err != nil {
			t.Fatal(err)
		}

		err := stor.AssignTasksToProject(ctx, []int{1, 999}, to.ID)
		if err == nil {
			t.Fatal("expected an error")
		}
		task, getErr := stor.GetTask(ctx, 1)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if task.ProjectID != from.ID {
			t.Fatalf("task changed to project %d", task.ProjectID)
		}
	})
}
