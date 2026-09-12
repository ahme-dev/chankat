package storage_test

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"chankat/internal/storage"
)

func TestOwnershipMigrationRepairsOnlyAttachedProjects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v4.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE TASK (ID INTEGER PRIMARY KEY, NAME TEXT, PROJECT_ID INTEGER, RATE_ID INTEGER);
		CREATE TABLE ENTRY (ID INTEGER PRIMARY KEY, TASK_ID INTEGER, PROJECT_ID INTEGER,
		RATE_ID INTEGER, STARTED_AT INTEGER, ENDED_AT INTEGER, NOTES TEXT);
		INSERT INTO TASK VALUES (1, 'moved', 2, NULL);
		INSERT INTO ENTRY VALUES (1, 1, 1, 7, 100, 3700, 'historical');
		INSERT INTO ENTRY VALUES (2, NULL, 1, 8, 100, 3700, 'detached');
		INSERT INTO ENTRY VALUES (3, 1, NULL, 7, 200, NULL, 'active');
		PRAGMA user_version = 4;
	`)
	if err != nil {
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
	for range 2 {
		if err := stor.Migrate(); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := stor.GetEntries(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{2, 1, 2} {
		if entries[i].ProjectID == nil || *entries[i].ProjectID != want {
			t.Fatalf("entry %d: %#v", i, entries[i])
		}
		var mirror int
		if err := stor.QueryRow(`SELECT PROJECT_ID FROM ENTRY WHERE ID = ?`, i+1).Scan(&mirror); err != nil {
			t.Fatal(err)
		}
		if mirror != want {
			t.Fatalf("mirror = %d, want %d", mirror, want)
		}
	}
	if *entries[0].RateID != 7 || entries[0].StartedAt.Unix() != 100 ||
		entries[0].EndedAt.Unix() != 3700 || entries[0].Note != "historical" ||
		*entries[1].RateID != 8 || entries[1].TaskID != nil || entries[2].EndedAt != nil {
		t.Fatalf("migration rewrote history: %#v", entries)
	}
}

func TestBulkTaskMoveAndArchivePreserveAccounting(t *testing.T) {
	stor := fixtureStorage(t)
	ctx := t.Context()
	from := fixtureProject(t, stor)
	to := fixtureProject(t, stor)
	start := time.Now().Add(-time.Hour).Truncate(time.Second)
	end := start.Add(30 * time.Minute)
	for _, name := range []string{"one", "two"} {
		if err := stor.CreateTaskAndEntry(ctx, storage.Task{Name: name, ProjectID: from.ID},
			storage.Entry{StartedAt: start, EndedAt: &end}); err != nil {
			t.Fatal(err)
		}
	}
	if err := stor.StartTask(ctx, 1, end); err != nil {
		t.Fatal(err)
	}
	if err := stor.AssignTasksToProject(ctx, []int{1, 999}, to.ID); err == nil {
		t.Fatal("missing task accepted")
	}
	if err := stor.AssignTasksToProject(ctx, []int{1, 2}, 999); err == nil {
		t.Fatal("missing project accepted")
	}
	entries, err := stor.GetEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if *entry.ProjectID != from.ID {
			t.Fatal("failed move changed attribution")
		}
	}
	if err := stor.AssignTasksToProject(ctx, []int{1, 2}, to.ID); err != nil {
		t.Fatal(err)
	}
	if err := stor.AssignEntriesToProject(ctx, []int{1}, &from.ID); err == nil {
		t.Fatal("attached entry allowed an independent project assignment")
	}
	if err := stor.DeleteTask(ctx, 1); err != nil {
		t.Fatal(err)
	}
	task, err := stor.GetTask(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !task.Archived {
		t.Fatal("task not archived")
	}
	visible, err := stor.GetTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 1 || visible[0].ID != 2 {
		t.Fatalf("visible tasks = %#v", visible)
	}
	if err := stor.StartTask(ctx, 1, time.Now()); err == nil {
		t.Fatal("archived task restarted")
	}
	entries, err = stor.GetEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.TaskID == nil || *entry.ProjectID != to.ID || *entry.RateID != from.RateID || entry.EndedAt == nil {
			t.Fatalf("lost archived history: %#v", entry)
		}
	}
	rates, err := stor.GetRates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	projects := storage.SummarizeProjects([]storage.Project{from, to}, rates, entries, nil, time.Now())
	if projects[0].BalanceMinor["USD"] != 0 || projects[1].BalanceMinor["USD"] < 11250 {
		t.Fatalf("wrong moved balances: %#v", projects)
	}
	// A stale entry editor must not put the work back on the previous project.
	entry := entries[0]
	entry.ProjectID = &from.ID
	entry.Note = "edited"
	if err := stor.UpdateEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}
	entry, err = stor.GetEntry(ctx, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if *entry.ProjectID != to.ID || entry.Note != "edited" {
		t.Fatalf("stale edit: %#v", entry)
	}
}

func TestOwnershipMigrationFailureRollsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken-v4.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an incomplete old database: ALTER TASK succeeds, backfill fails.
	if _, err := db.Exec(`CREATE TABLE TASK (ID INTEGER PRIMARY KEY, PROJECT_ID INTEGER); PRAGMA user_version = 4;`); err != nil {
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
	if err := stor.Migrate(); err == nil {
		t.Fatal("invalid migration succeeded")
	}
	var version, archivedColumns int
	if err := stor.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := stor.QueryRow(`SELECT count(*) FROM pragma_table_info('TASK') WHERE name = 'ARCHIVED'`).Scan(&archivedColumns); err != nil {
		t.Fatal(err)
	}
	if version != 4 || archivedColumns != 0 {
		t.Fatalf("partial migration committed: version=%d, archived=%d", version, archivedColumns)
	}
}

func TestLedgerBreakdownKeepsCreditsCurrenciesAndRounding(t *testing.T) {
	projectID, usdID, eurID := 1, 1, 2
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	entries := []storage.Entry{
		{ProjectID: &projectID, RateID: &usdID, StartedAt: start, EndedAt: &end},
		{ProjectID: &projectID, RateID: &eurID, StartedAt: start, EndedAt: &end},
	}
	payments := []storage.Payment{
		{ProjectID: projectID, Currency: "USD", AmountMinor: 50, PaidAt: start},
		{ProjectID: projectID, Currency: "EUR", AmountMinor: 100, PaidAt: start},
		{ProjectID: projectID, Currency: "USD", AmountMinor: 1000, PaidAt: start.AddDate(0, 0, 1)},
	}
	got := storage.SummarizeProjects([]storage.Project{{ID: projectID, RateID: usdID}},
		[]storage.Rate{{ID: usdID, AmountMinor: 101, Currency: "USD"}, {ID: eurID, AmountMinor: 3, Currency: "EUR"}},
		entries, payments, start.Add(30*time.Minute))[0]
	if got.EarnedMinor["USD"] != 50 || got.PaidMinor["USD"] != 50 || got.BalanceMinor["USD"] != 0 ||
		got.EarnedMinor["EUR"] != 1 || got.PaidMinor["EUR"] != 100 || got.BalanceMinor["EUR"] != -99 {
		t.Fatalf("ledger = %#v", got)
	}
	for currency, balance := range got.BalanceMinor {
		if got.EarnedMinor[currency]-got.PaidMinor[currency] != balance {
			t.Fatalf("unreconciled %s", currency)
		}
	}
}

func TestProjectLedgerUsesHistoricalRatesAndCountsUnratedTime(t *testing.T) {
	projectID, oldRate, newRate, taskID := 1, 1, 2, 1
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first, second, third := start.Add(time.Hour), start.Add(2*time.Hour), start.Add(3*time.Hour)
	entries := []storage.Entry{
		{TaskID: &taskID, ProjectID: &projectID, RateID: &oldRate, StartedAt: start, EndedAt: &first},
		{TaskID: &taskID, ProjectID: &projectID, RateID: &newRate, StartedAt: first, EndedAt: &second},
		{TaskID: &taskID, ProjectID: &projectID, StartedAt: second, EndedAt: &third},
	}
	rates := []storage.Rate{{ID: oldRate, AmountMinor: 10000, Currency: "USD"}, {ID: newRate, AmountMinor: 20000, Currency: "USD"}}
	projects := []storage.Project{{ID: projectID, RateID: newRate}}
	payments := []storage.Payment{{ProjectID: projectID, Currency: "USD", AmountMinor: 15000, PaidAt: start}}
	got := storage.SummarizeProjects(projects, rates, entries, payments, third)[0]
	if got.Tracked != 3*time.Hour || got.EarnedMinor["USD"] != 30000 || got.PaidMinor["USD"] != 15000 || got.BalanceMinor["USD"] != 15000 {
		t.Fatalf("ledger = %#v", got)
	}
	task := storage.SummarizeTasks([]storage.Task{{ID: taskID, ProjectID: projectID}}, projects, entries, rates, third)[0]
	if task.Rate.ID != newRate || len(task.HistoricalRates) != 2 || task.EarnedMinor["USD"] != 30000 {
		t.Fatalf("task = %#v", task)
	}
}
