package storage_test

import (
	"strings"
	"testing"
	"time"

	"chansat/internal/storage"
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
