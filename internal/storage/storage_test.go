package storage_test

import (
	"testing"

	"chansat/internal/storage"
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
