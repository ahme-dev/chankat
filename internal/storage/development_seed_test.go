package storage_test

import (
	"strings"
	"testing"
	"time"

	"chankat/internal/storage"
)

func TestSeedDevelopment(t *testing.T) {
	stor := fixtureStorage(t)
	now := time.Date(2026, 8, 9, 16, 0, 0, 0, time.UTC)

	if err := stor.SeedDevelopment(t.Context(), now, false); err != nil {
		t.Fatal(err)
	}
	assertSeedCounts(t, stor, 3, 3, 6, 8, 3)

	if err := stor.SeedDevelopment(t.Context(), now, false); err == nil ||
		!strings.Contains(err.Error(), "database is not empty") {
		t.Fatalf("second seed error = %v", err)
	}
	if err := stor.SeedDevelopment(t.Context(), now.AddDate(0, 0, 1), true); err != nil {
		t.Fatal(err)
	}
	assertSeedCounts(t, stor, 3, 3, 6, 8, 3)

	active, err := stor.GetActiveEntries(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].Note != "Investigating token refresh" {
		t.Fatalf("active entries = %#v", active)
	}
}

func assertSeedCounts(
	t *testing.T,
	stor *storage.Storage,
	rates, projects, tasks, entries, payments int,
) {
	t.Helper()
	queries := []struct {
		table string
		want  int
	}{
		{"RATE", rates},
		{"PROJECT", projects},
		{"TASK", tasks},
		{"ENTRY", entries},
		{"PAYMENT", payments},
	}
	for _, query := range queries {
		var got int
		if err := stor.QueryRow("SELECT COUNT(*) FROM " + query.table).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != query.want {
			t.Errorf("%s count = %d, want %d", query.table, got, query.want)
		}
	}
}
