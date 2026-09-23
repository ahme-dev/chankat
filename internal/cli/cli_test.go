package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"chankat/internal/storage"
)

func TestHelpAndVersionDoNotNeedStorage(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"help"}, want: "Usage:"},
		{args: []string{"rates", "create", "--help"}, want: "--amount-minor"},
		{args: []string{"version"}, want: "chankat test"},
		{args: []string{"--json", "version"}, want: `"version": "test"`},
	} {
		var out bytes.Buffer
		if err := RunIO(
			t.Context(), test.args, "test", nil, &out, &out,
		); err != nil {
			t.Fatalf("%v: %v", test.args, err)
		}
		if !strings.Contains(out.String(), test.want) {
			t.Fatalf("%v output %q does not contain %q", test.args, out.String(), test.want)
		}
		if RequiresStorage(test.args) {
			t.Fatalf("%v unexpectedly requires storage", test.args)
		}
	}
}

func TestCLIResourceWorkflow(t *testing.T) {
	stor := cliStorage(t)

	created := runCLI(t, stor, "--json", "rates", "create", "--name", "Standard",
		"--amount-minor", "10000", "--currency", "usd")
	if !strings.Contains(created, `"id": 1`) {
		t.Fatalf("create output lacks ID: %q", created)
	}
	runCLI(t, stor, "rates", "update", "1", "--name", "Consulting")
	runCLI(t, stor, "projects", "create", "--name", "Acme", "--rate", "1")
	runCLI(t, stor, "projects", "update", "1", "--name", "Acme Corp")
	runCLI(t, stor, "tasks", "create", "--name", "Build", "--project", "1")
	runCLI(t, stor, "tasks", "update", "1", "--name", "Build CLI")
	runCLI(t, stor, "tasks", "start", "1", "--at", "2026-07-30T09:00:00Z")
	runCLIAt(
		t,
		stor,
		time.Date(2026, 7, 30, 10, 30, 0, 0, time.UTC),
		"tasks", "stop", "1",
	)
	runCLI(t, stor, "entries", "update", "1", "--note", "first pass")
	runCLI(t, stor, "entries", "create", "--task", "1",
		"--started-at", "2026-07-30T11:00:00Z",
		"--ended-at", "2026-07-30T12:00:00Z", "--note", "second pass")
	runCLI(t, stor, "payments", "create", "--project", "1",
		"--amount-minor", "5000", "--currency", "usd",
		"--paid-at", "2026-07-30", "--paid-for", "2026-06-01")
	runCLI(t, stor, "payments", "update", "1", "--note", "deposit",
		"--paid-for", "discarded-value")

	var rate rateOutput
	decodeCLI(t, stor, &rate, "--json", "rates", "get", "1")
	if rate.Name != "Consulting" || rate.ProjectCount != 1 {
		t.Fatalf("unexpected rate output: %#v", rate)
	}

	var tasks []taskOutput
	decodeCLI(t, stor, &tasks, "--json", "tasks", "list")
	if len(tasks) != 1 || tasks[0].Name != "Build CLI" ||
		tasks[0].TrackedSeconds != 9_000 ||
		tasks[0].EarnedMinor["USD"] != 25_000 {
		t.Fatalf("unexpected task output: %#v", tasks)
	}

	var projects []projectOutput
	decodeCLI(t, stor, &projects, "--json", "projects")
	if len(projects) != 1 || projects[0].TrackedSeconds != 9_000 ||
		projects[0].BalanceMinor["USD"] != 20_000 {
		t.Fatalf("unexpected project output: %#v", projects)
	}
	var project projectOutput
	decodeCLI(t, stor, &project, "--json", "projects", "get", "1")
	if project.Name != "Acme Corp" {
		t.Fatalf("unexpected project: %#v", project)
	}

	var entries []entryOutput
	decodeCLI(t, stor, &entries, "--json", "entries", "list", "--task", "1")
	if len(entries) != 2 || entries[0].Note != "first pass" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	var entry entryOutput
	decodeCLI(t, stor, &entry, "--json", "entries", "get", "1")
	if entry.Note != "first pass" {
		t.Fatalf("unexpected entry: %#v", entry)
	}

	var payment paymentOutput
	decodeCLI(t, stor, &payment, "--json", "payments", "get", "1")
	if payment.ProjectName != "Acme Corp" || payment.Note != "deposit" ||
		payment.PaidAt != "2026-07-30" {
		t.Fatalf("unexpected payment: %#v", payment)
	}

	runCLI(t, stor, "payments", "delete", "1")
	runCLI(t, stor, "entries", "delete", "1")
	runCLI(t, stor, "entries", "delete", "2")
	runCLI(t, stor, "tasks", "delete", "1")
	runCLI(t, stor, "projects", "delete", "1")
	runCLI(t, stor, "rates", "delete", "1")
}

func TestCLICreatesHistoricalTaskAndActiveEntry(t *testing.T) {
	stor := cliStorage(t)
	runCLI(t, stor, "rates", "create", "--name", "Rate",
		"--amount-minor", "8000", "--currency", "EUR")
	runCLI(t, stor, "projects", "create", "--name", "Project", "--rate", "1")
	runCLI(t, stor, "tasks", "create", "--name", "Past", "--project", "1",
		"--started-at", "2026-07-30 09:00",
		"--ended-at", "2026-07-30 10:00", "--note", "imported")
	runCLI(t, stor, "tasks", "create", "--name", "Current", "--project", "1",
		"--start")

	entries, err := stor.GetEntries(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].EndedAt == nil || entries[1].EndedAt != nil {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

func TestCLITaskRateOverrideCanBeChangedAndCleared(t *testing.T) {
	stor := cliStorage(t)
	for _, args := range [][]string{
		{"rates", "create", "--name", "First", "--amount-minor", "10000", "--currency", "USD"},
		{"rates", "create", "--name", "Second", "--amount-minor", "20000", "--currency", "USD"},
		{"rates", "create", "--name", "Override", "--amount-minor", "30000", "--currency", "USD"},
		{"projects", "create", "--name", "One", "--rate", "1"},
		{"projects", "create", "--name", "Two", "--rate", "2"},
		{"tasks", "create", "--name", "Task", "--project", "1", "--rate", "3"},
	} {
		runCLI(t, stor, args...)
	}

	var task taskOutput
	decodeCLI(t, stor, &task, "--json", "tasks", "get", "1")
	if task.RateID != 3 || task.RateAmountMinor != 30_000 ||
		!task.RateOverridden {
		t.Fatalf("task override output = %#v", task)
	}

	runCLI(t, stor, "tasks", "update", "1", "--project", "2", "--rate", "0")
	decodeCLI(t, stor, &task, "--json", "tasks", "get", "1")
	if task.ProjectID != 2 || task.RateID != 2 ||
		task.RateAmountMinor != 20_000 || task.RateOverridden {
		t.Fatalf("inherited task output = %#v", task)
	}
}

func TestCLIEntryRateCanBeChangedAndCleared(t *testing.T) {
	stor := cliStorage(t)
	for _, args := range [][]string{
		{"rates", "create", "--name", "Old", "--amount-minor", "10000", "--currency", "USD"},
		{"rates", "create", "--name", "Corrected", "--amount-minor", "25000", "--currency", "USD"},
		{"projects", "create", "--name", "Project", "--rate", "1"},
		{"tasks", "create", "--name", "Task", "--project", "1", "--started-at", "2026-01-01 09:00", "--ended-at", "2026-01-01 10:00"},
	} {
		runCLI(t, stor, args...)
	}

	runCLI(t, stor, "entries", "update", "1", "--rate", "2")
	var entry entryOutput
	decodeCLI(t, stor, &entry, "--json", "entries", "get", "1")
	if entry.RateID == nil || *entry.RateID != 2 {
		t.Fatalf("updated entry rate = %#v", entry.RateID)
	}
	var projects []projectOutput
	decodeCLI(t, stor, &projects, "--json", "projects", "list")
	if projects[0].EarnedMinor["USD"] != 25_000 {
		t.Fatalf("earnings after rate correction = %#v", projects[0].EarnedMinor)
	}

	runCLI(t, stor, "entries", "update", "1", "--rate", "0")
	decodeCLI(t, stor, &entry, "--json", "entries", "get", "1")
	if entry.RateID != nil {
		t.Fatalf("cleared entry rate = %#v", entry.RateID)
	}
}

func TestCLIProjectMoveAndArchiveKeepHistoricalEarnings(t *testing.T) {
	stor := cliStorage(t)
	for _, args := range [][]string{
		{"rates", "create", "--name", "Old", "--amount-minor", "10000", "--currency", "USD"},
		{"rates", "create", "--name", "New", "--amount-minor", "20000", "--currency", "USD"},
		{"projects", "create", "--name", "One", "--rate", "1"},
		{"projects", "create", "--name", "Two", "--rate", "2"},
		{"tasks", "create", "--name", "History", "--project", "1", "--started-at", "2026-01-01 09:00", "--ended-at", "2026-01-01 10:00"},
		{"tasks", "update", "1", "--project", "2"},
		{"payments", "create", "--project", "2", "--amount-minor", "4000", "--currency", "USD", "--paid-at", "2026-01-01"},
		{"tasks", "delete", "1"},
	} {
		runCLI(t, stor, args...)
	}
	var projects []projectOutput
	decodeCLI(t, stor, &projects, "--json", "projects", "list")
	if projects[0].TrackedSeconds != 0 || projects[0].BalanceMinor["USD"] != 0 ||
		projects[1].EarnedMinor["USD"] != 10000 || projects[1].PaidMinor["USD"] != 4000 || projects[1].BalanceMinor["USD"] != 6000 {
		t.Fatalf("projects = %#v", projects)
	}
	var tasks []taskOutput
	decodeCLI(t, stor, &tasks, "--json", "tasks", "list")
	if len(tasks) != 0 {
		t.Fatal("archived task visible")
	}
	decodeCLI(t, stor, &tasks, "--json", "tasks", "list", "--archived")
	if len(tasks) != 1 || !tasks[0].Archived || tasks[0].RateID != 2 ||
		len(tasks[0].HistoricalRates) != 1 || tasks[0].HistoricalRates[0].ID != 1 || tasks[0].EarnedMinor["USD"] != 10000 {
		t.Fatalf("archived history = %#v", tasks)
	}
	text := runCLI(t, stor, "tasks", "get", "1")
	if !strings.Contains(text, "NEXT_RATE") || !strings.Contains(text, "1:USD:10000") {
		t.Fatalf("task output: %s", text)
	}
	runCLI(t, stor, "tasks", "restore", "1")
	decodeCLI(t, stor, &tasks, "--json", "tasks", "list")
	if len(tasks) != 1 || tasks[0].Archived || tasks[0].Active {
		t.Fatalf("restored task = %#v", tasks)
	}
}

func TestCLIStopsAllTasks(t *testing.T) {
	stor := cliStorage(t)
	runCLI(t, stor, "rates", "create", "--name", "Rate",
		"--amount-minor", "8000", "--currency", "EUR")
	runCLI(t, stor, "projects", "create", "--name", "Project", "--rate", "1")
	for _, name := range []string{"First", "Second"} {
		runCLI(t, stor, "tasks", "create", "--name", name, "--project", "1",
			"--started-at", "2026-07-30T09:00:00Z")
	}
	runCLI(t, stor, "tasks", "start", "1", "--at", "2026-07-30T09:30:00Z")

	var result struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	output := runCLIAt(
		t,
		stor,
		time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC),
		"--json", "tasks", "stop", "--all",
	)
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode stop-all output %q: %v", output, err)
	}
	if result.Status != "stopped" || result.Count != 2 {
		t.Fatalf("unexpected stop result: %#v", result)
	}
	active, err := stor.GetActiveEntries(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("%d entries remain active", len(active))
	}
}

func TestCLIDashboardCustomPeriod(t *testing.T) {
	stor := cliStorage(t)
	runCLI(t, stor, "rates", "create", "--name", "Rate",
		"--amount-minor", "10000", "--currency", "USD")
	runCLI(t, stor, "projects", "create", "--name", "Acme", "--rate", "1")
	runCLI(t, stor, "tasks", "create", "--name", "Build", "--project", "1",
		"--started-at", "2026-08-02 23:00",
		"--ended-at", "2026-08-03 01:00")
	runCLI(t, stor, "payments", "create", "--project", "1",
		"--amount-minor", "20000", "--currency", "USD",
		"--paid-at", "2026-08-09")

	output := runCLIAt(t, stor, time.Date(2026, 8, 9, 12, 0, 0, 0, time.Local),
		"--json", "dashboard", "--from", "2026-08-03", "--to", "2026-08-03")
	var got dashboardOutput
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatal(err)
	}
	if got.Period != "custom" || got.TrackedSeconds != 3600 ||
		got.EarnedMinor["USD"] != 10_000 || got.PaidMinor["USD"] != 0 ||
		got.BalanceMinor["USD"] != 0 || len(got.Projects) != 1 ||
		got.Projects[0].BalanceMinor["USD"] != 0 {
		t.Fatalf("dashboard = %#v", got)
	}
}

func TestCLIExportsSingleProjectTaskEntries(t *testing.T) {
	stor := cliStorage(t)
	for _, args := range [][]string{
		{"rates", "create", "--name", "Rate", "--amount-minor", "10000", "--currency", "USD"},
		{"projects", "create", "--name", "Acme", "--rate", "1"},
		{"projects", "create", "--name", "Other", "--rate", "1"},
		{"tasks", "create", "--name", "Build, test", "--project", "1", "--started-at", "2026-08-03 09:00", "--ended-at", "2026-08-03 10:30"},
		{"tasks", "start", "1", "--at", "2026-08-03 11:00"},
		{"tasks", "create", "--name", "Archived", "--project", "1", "--started-at", "2026-08-02 15:00", "--ended-at", "2026-08-02 15:30"},
		{"tasks", "delete", "2"},
		{"tasks", "create", "--name", "Ignore", "--project", "2", "--started-at", "2026-08-03 09:00", "--ended-at", "2026-08-03 10:00"},
	} {
		runCLI(t, stor, args...)
	}

	now := time.Date(2026, 8, 3, 11, 45, 0, 0, time.Local)
	output := runCLIAt(t, stor, now,
		"projects", "export", "1")
	records, err := csv.NewReader(strings.NewReader(output)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"task_name", "working_minutes", "started_at", "ended_at"},
		{"Build, test", "90.00", localRFC3339(2026, 8, 3, 9, 0), localRFC3339(2026, 8, 3, 10, 30)},
		{"Build, test", "45.00", localRFC3339(2026, 8, 3, 11, 0), localRFC3339(2026, 8, 3, 11, 45)},
		{"Archived", "30.00", localRFC3339(2026, 8, 2, 15, 0), localRFC3339(2026, 8, 2, 15, 30)},
	}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("export records = %#v, want %#v", records, want)
	}
}

func localRFC3339(year int, month time.Month, day, hour, minute int) string {
	return time.Date(year, month, day, hour, minute, 0, 0, time.Local).Format(time.RFC3339)
}

func TestCLIProjectExportRejectsMissingProject(t *testing.T) {
	stor := cliStorage(t)
	var out bytes.Buffer
	err := RunIO(t.Context(), []string{"projects", "export", "99"}, "test", stor, &out, &out)
	if err == nil || !strings.Contains(err.Error(), "project 99 not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLIDashboardRejectsInvalidRanges(t *testing.T) {
	stor := cliStorage(t)
	for _, args := range [][]string{
		{"dashboard", "--from", "2026-08-01"},
		{"dashboard", "--period", "week", "--from", "2026-08-01", "--to", "2026-08-02"},
		{"dashboard", "--from", "2026-08-02", "--to", "2026-08-01"},
		{"dashboard", "--period", "quarter"},
	} {
		var out bytes.Buffer
		if err := RunIO(t.Context(), args, "test", stor, &out, &out); err == nil {
			t.Fatalf("%v succeeded", args)
		}
	}
}

func TestCLIRejectsInvalidArguments(t *testing.T) {
	stor := cliStorage(t)
	for _, args := range [][]string{
		{"rates", "create", "--name", "missing fields"},
		{"tasks", "get", "zero"},
		{"entries", "create", "--task", "1", "--started-at", "bad"},
		{"payments", "create", "--project", "1", "--amount-minor", "1",
			"--currency", "USD", "--paid-at", "bad"},
		{"unknown"},
	} {
		var out bytes.Buffer
		if err := RunIO(t.Context(), args, "test", stor, &out, &out); err == nil {
			t.Fatalf("%v succeeded", args)
		}
	}
}

func TestRatesCompatibilityAlias(t *testing.T) {
	stor := cliStorage(t)
	runCLI(t, stor, "rates", "create", "--name", "Rate",
		"--amount-minor", "1", "--currency", "USD")
	output := runCLI(t, stor, "rates")
	if !strings.Contains(output, "AMOUNT_MINOR") || !strings.Contains(output, "USD") {
		t.Fatalf("unexpected output: %q", output)
	}
}

func TestFormatTracked(t *testing.T) {
	for _, test := range []struct {
		seconds int64
		want    string
	}{
		{seconds: 0, want: "0s"},
		{seconds: 45, want: "45s"},
		{seconds: 90, want: "1m 30s"},
		{seconds: 5_400, want: "1h 30m"},
	} {
		if got := formatTracked(test.seconds); got != test.want {
			t.Errorf("formatTracked(%d) = %q, want %q", test.seconds, got, test.want)
		}
	}
}

func cliStorage(t *testing.T) *storage.Storage {
	t.Helper()
	t.Setenv("CHANKAT_DATA_PATH", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	stor, err := storage.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stor.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := stor.Migrate(); err != nil {
		t.Fatal(err)
	}
	return stor
}

func runCLI(t *testing.T, stor *storage.Storage, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	if err := RunIO(t.Context(), args, "test", stor, &out, &out); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

func runCLIAt(
	t *testing.T,
	stor *storage.Storage,
	now time.Time,
	args ...string,
) string {
	t.Helper()
	filtered, jsonOutput := globalOptions(args)
	var out bytes.Buffer
	r := runner{
		ctx: t.Context(), stor: stor, version: "test", out: &out, errOut: &out,
		json: jsonOutput, now: func() time.Time { return now },
	}
	if err := r.run(filtered); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

func decodeCLI(
	t *testing.T,
	stor *storage.Storage,
	target any,
	args ...string,
) {
	t.Helper()
	output := runCLI(t, stor, args...)
	if err := json.Unmarshal([]byte(output), target); err != nil {
		t.Fatalf("decode %v output %q: %v", args, output, err)
	}
}
