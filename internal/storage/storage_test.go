package storage_test

import (
	"testing"

	"chansat/internal/storage"
)

func TestOpenAndMigrate(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	db, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
	})

	if err := storage.Migrate(db); err != nil {
		t.Fatal(err)
	}

	var count int
	err = db.QueryRow(`
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
