package storage_test

import (
	"strings"
	"testing"

	"chankat/internal/storage"
)

func TestGetProjects(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		stor := fixtureStorage(t)
		projects, err := stor.GetProjects(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(projects) != 0 {
			t.Fatalf("got %d projects, want 0", len(projects))
		}
	})

	t.Run("returns projects", func(t *testing.T) {
		stor := fixtureStorage(t)
		want := fixtureProject(t, stor)
		projects, err := stor.GetProjects(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(projects) != 1 {
			t.Fatalf("got %d projects, want 1", len(projects))
		}
		assertProject(t, projects[0], want)
	})
}

func TestGetProject(t *testing.T) {
	t.Run("existing project", func(t *testing.T) {
		stor := fixtureStorage(t)
		want := fixtureProject(t, stor)
		got, err := stor.GetProject(t.Context(), want.ID)
		if err != nil {
			t.Fatal(err)
		}
		assertProject(t, got, want)
	})

	t.Run("missing project", func(t *testing.T) {
		stor := fixtureStorage(t)
		_, err := stor.GetProject(t.Context(), 999)
		if err == nil || !strings.Contains(err.Error(), "project 999 not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestCreateProject(t *testing.T) {
	t.Run("requires existing rate", func(t *testing.T) {
		stor := fixtureStorage(t)
		err := stor.CreateProject(t.Context(), storage.Project{Name: "Acme", RateID: 999})
		if err == nil {
			t.Fatal("expected a foreign-key error")
		}
	})
}

func TestUpdateProject(t *testing.T) {
	t.Run("updates existing project", func(t *testing.T) {
		stor := fixtureStorage(t)
		want := fixtureProject(t, stor)
		want.Name = "Acme revised"

		if err := stor.UpdateProject(t.Context(), want); err != nil {
			t.Fatal(err)
		}
		got, err := stor.GetProject(t.Context(), want.ID)
		if err != nil {
			t.Fatal(err)
		}
		assertProject(t, got, want)
	})

	t.Run("missing project", func(t *testing.T) {
		stor := fixtureStorage(t)
		err := stor.UpdateProject(t.Context(), storage.Project{
			ID: 999, Name: "missing", RateID: fixtureRate(t, stor).ID,
		})
		if err == nil || !strings.Contains(err.Error(), "project 999 not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDeleteProject(t *testing.T) {
	t.Run("existing project", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		if err := stor.DeleteProject(t.Context(), project.ID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing project", func(t *testing.T) {
		stor := fixtureStorage(t)
		err := stor.DeleteProject(t.Context(), 999)
		if err == nil || !strings.Contains(err.Error(), "project 999 not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func assertProject(t *testing.T, got, want storage.Project) {
	t.Helper()
	if got.ID != want.ID || got.Name != want.Name || got.RateID != want.RateID {
		t.Errorf("got project %#v, want %#v", got, want)
	}
}
