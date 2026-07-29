package storage_test

import (
	"testing"

	"chankat/internal/storage"
)

func TestOpenAndMigrate(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	stor, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stor.Close()
	})

	if err := stor.Migrate(); err != nil {
		t.Fatal(err)
	}

	var count int
	err = stor.QueryRow(`
		SELECT count(*)
		FROM sqlite_master
		WHERE type = 'table'
		  AND name IN ('RATE', 'PROJECT', 'TASK', 'ENTRY', 'PAYMENT')
	`).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != 5 {
		t.Fatalf("got %d tables, want 5", count)
	}

	if err := stor.Migrate(); err != nil {
		t.Fatalf("second migration: %v", err)
	}

	var paidForColumn int
	err = stor.QueryRow(`
		SELECT count(*)
		FROM pragma_table_info('PAYMENT')
		WHERE name = 'PAID_FOR_DATE'
	`).Scan(&paidForColumn)
	if err != nil {
		t.Fatal(err)
	}
	if paidForColumn != 1 {
		t.Fatalf("got %d PAID_FOR_DATE columns, want 1", paidForColumn)
	}

	var taskProjectRequired int
	err = stor.QueryRow(`
		SELECT "notnull"
		FROM pragma_table_info('TASK')
		WHERE name = 'PROJECT_ID'
	`).Scan(&taskProjectRequired)
	if err != nil {
		t.Fatal(err)
	}
	if taskProjectRequired != 1 {
		t.Fatal("TASK.PROJECT_ID is nullable")
	}
}

func fixtureStorage(t *testing.T) *storage.Storage {
	t.Helper()

	t.Setenv("XDG_DATA_HOME", t.TempDir())

	stor, err := storage.Open()
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}

	t.Cleanup(func() {
		if err := stor.Close(); err != nil {
			t.Errorf("failed to close storage: %v", err)
		}
	})

	if err := stor.Migrate(); err != nil {
		t.Fatalf("failed to migrate storage: %v", err)
	}

	return stor
}

func fixtureRate(t *testing.T, stor *storage.Storage) storage.Rate {
	t.Helper()

	rate := storage.Rate{Name: "standard", AmountMinor: 7500, Currency: "USD"}
	if err := stor.CreateRate(t.Context(), rate); err != nil {
		t.Fatalf("create fixture rate: %v", err)
	}
	rates, err := stor.GetRates(t.Context())
	if err != nil {
		t.Fatalf("get fixture rate: %v", err)
	}
	return rates[len(rates)-1]
}

func fixtureProject(t *testing.T, stor *storage.Storage) storage.Project {
	t.Helper()

	rate := fixtureRate(t, stor)
	project := storage.Project{Name: "Acme", RateID: rate.ID}
	if err := stor.CreateProject(t.Context(), project); err != nil {
		t.Fatalf("create fixture project: %v", err)
	}
	projects, err := stor.GetProjects(t.Context())
	if err != nil {
		t.Fatalf("get fixture project: %v", err)
	}
	return projects[len(projects)-1]
}

func intPointer(value int) *int {
	return &value
}
