package storage_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"chankat/internal/storage"
)

func TestMigrationBackfillsLegacyPaymentDate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	legacyPaidAt := time.Date(2024, 2, 1, 0, 0, 0, 0, time.Local).Unix()
	paidAt := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC).Unix()
	if _, err := db.Exec(`
		CREATE TABLE PAYMENT (
			ID INTEGER PRIMARY KEY,
			PROJECT_ID INTEGER NOT NULL,
			AMOUNT_MINOR INTEGER NOT NULL,
			CURRENCY TEXT NOT NULL,
			PAID_AT INTEGER NOT NULL,
			NOTES TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO PAYMENT (
			ID, PROJECT_ID, AMOUNT_MINOR, CURRENCY, PAID_AT
		) VALUES (1, 1, 5000, 'USD', ?);
		PRAGMA user_version = 1;
	`, legacyPaidAt); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CHANKAT_DATA_PATH", path)
	stor, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stor.Close() })
	if err := stor.Migrate(); err != nil {
		t.Fatal(err)
	}
	var got int64
	if err := stor.QueryRow(
		`SELECT PAID_FOR_DATE FROM PAYMENT WHERE ID = 1`,
	).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != paidAt {
		t.Fatalf("paid-for compatibility date = %d, want %d", got, paidAt)
	}
}

func TestMigrationReplacesExistingPaymentAccountingDate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version-two.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	const (
		paidFor = int64(1_704_067_200)
	)
	legacyPaidAt := time.Date(2024, 2, 1, 0, 0, 0, 0, time.Local).Unix()
	paidAt := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC).Unix()
	if _, err := db.Exec(`
		CREATE TABLE PAYMENT (
			ID INTEGER PRIMARY KEY,
			PROJECT_ID INTEGER NOT NULL,
			AMOUNT_MINOR INTEGER NOT NULL,
			CURRENCY TEXT NOT NULL,
			PAID_AT INTEGER NOT NULL,
			PAID_FOR_DATE INTEGER NOT NULL DEFAULT 0,
			NOTES TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO PAYMENT (
			ID, PROJECT_ID, AMOUNT_MINOR, CURRENCY, PAID_AT, PAID_FOR_DATE
			) VALUES (1, 1, 5000, 'USD', ?, ?);
			PRAGMA user_version = 2;
	`, legacyPaidAt, paidFor); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CHANKAT_DATA_PATH", path)
	stor, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stor.Close() })
	if err := stor.Migrate(); err != nil {
		t.Fatal(err)
	}
	var got int64
	if err := stor.QueryRow(
		`SELECT PAID_FOR_DATE FROM PAYMENT WHERE ID = 1`,
	).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != paidAt {
		t.Fatalf("migrated compatibility date = %d, want %d", got, paidAt)
	}
}

func TestOpenAndMigrate(t *testing.T) {
	t.Setenv("CHANKAT_DATA_PATH", "")
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

func TestOpenUsesConfiguredDataPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "development.sqlite")
	t.Setenv("CHANKAT_DATA_PATH", path)
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "ignored"))

	stor, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := stor.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("configured database was not created: %v", err)
	}
}

func fixtureStorage(t *testing.T) *storage.Storage {
	t.Helper()

	t.Setenv("CHANKAT_DATA_PATH", "")
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
