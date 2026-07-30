package cli

import (
	"fmt"
	"strings"
	"time"

	"chankat/internal/storage"
)

func (r runner) runTasks(args []string) error {
	if len(args) == 0 {
		return r.listTasks(nil)
	}
	switch args[0] {
	case "list":
		return r.listTasks(args[1:])
	case "get":
		return r.getTask(args[1:])
	case "create":
		return r.createTask(args[1:])
	case "update":
		return r.updateTask(args[1:])
	case "delete":
		return r.deleteTask(args[1:])
	case "start":
		return r.startTask(args[1:])
	case "help", "-h", "--help":
		r.help([]string{"tasks"})
		return nil
	default:
		return fmt.Errorf("unknown tasks command %q", args[0])
	}
}

func (r runner) loadTasks() ([]storage.TaskSummary, error) {
	tasks, err := r.stor.GetTasks(r.ctx)
	if err != nil {
		return nil, err
	}
	projects, err := r.stor.GetProjects(r.ctx)
	if err != nil {
		return nil, err
	}
	entries, err := r.stor.GetEntries(r.ctx)
	if err != nil {
		return nil, err
	}
	rates, err := r.stor.GetRates(r.ctx)
	if err != nil {
		return nil, err
	}
	return storage.SummarizeTasks(tasks, projects, entries, rates, r.now()), nil
}

func (r runner) listTasks(args []string) error {
	flags := r.flags("tasks", "list")
	activeOnly := flags.Bool("active", false, "show only actively tracked tasks")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	items, err := r.loadTasks()
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}
	if *activeOnly {
		filtered := items[:0]
		for _, item := range items {
			if item.Active {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	output := taskOutputs(items)
	if r.json {
		return r.writeJSON(output)
	}
	rows := make([]string, len(output))
	for i, item := range output {
		rows[i] = fmt.Sprintf("%d\t%s\t%d\t%s\t%t\t%s\t%s", item.ID,
			item.Name, item.ProjectID, item.ProjectName, item.Active,
			formatTracked(item.TrackedSeconds), formatMinorMap(item.EarnedMinor))
	}
	return r.table(
		"ID\tNAME\tPROJECT_ID\tPROJECT\tACTIVE\tTRACKED\tEARNED_MINOR",
		rows,
	)
}

func (r runner) getTask(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: chankat tasks get ID")
	}
	id, err := parseID(args[0], "task")
	if err != nil {
		return err
	}
	items, err := r.loadTasks()
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	for _, item := range items {
		if item.ID == id {
			output := taskOutputs([]storage.TaskSummary{item})[0]
			if r.json {
				return r.writeJSON(output)
			}
			return r.table(
				"ID\tNAME\tPROJECT_ID\tPROJECT\tACTIVE\tTRACKED\tEARNED_MINOR",
				[]string{fmt.Sprintf("%d\t%s\t%d\t%s\t%t\t%s\t%s", output.ID,
					output.Name, output.ProjectID, output.ProjectName,
					output.Active, formatTracked(output.TrackedSeconds),
					formatMinorMap(output.EarnedMinor))},
			)
		}
	}
	return fmt.Errorf("task %d not found", id)
}

func (r runner) createTask(args []string) error {
	flags := r.flags("tasks", "create")
	name := flags.String("name", "", "task name")
	projectID := flags.Int("project", 0, "project ID")
	start := flags.Bool("start", false, "start tracking now")
	startedAt := flags.String("started-at", "", "RFC3339 or YYYY-MM-DD HH:MM")
	endedAt := flags.String("ended-at", "", "RFC3339 or YYYY-MM-DD HH:MM")
	note := flags.String("note", "", "entry note")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if err := required(flags, "name", "project"); err != nil {
		return err
	}
	task := storage.Task{Name: *name, ProjectID: *projectID}
	hasEntry := *start || changed(flags, "started-at") || changed(flags, "ended-at")
	if !hasEntry {
		if changed(flags, "note") {
			return fmt.Errorf("--note requires an entry")
		}
		id, err := r.stor.CreateTaskID(r.ctx, task)
		if err != nil {
			return err
		}
		return r.status("created", "task", id)
	}
	if *start && changed(flags, "started-at") {
		return fmt.Errorf("--start and --started-at cannot be used together")
	}
	started := r.now()
	var err error
	if changed(flags, "started-at") {
		started, err = parseDateTime(*startedAt)
		if err != nil {
			return err
		}
	}
	var ended *time.Time
	if changed(flags, "ended-at") {
		if !changed(flags, "started-at") {
			return fmt.Errorf("--ended-at requires --started-at")
		}
		value, err := parseDateTime(*endedAt)
		if err != nil {
			return err
		}
		ended = &value
	}
	id, err := r.stor.CreateTaskAndEntryID(r.ctx, task, storage.Entry{
		StartedAt: started, EndedAt: ended, Note: *note,
	})
	if err != nil {
		return err
	}
	return r.status("created", "task", id)
}

func (r runner) updateTask(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: chankat tasks update ID [options]")
	}
	id, err := parseID(args[0], "task")
	if err != nil {
		return err
	}
	task, err := r.stor.GetTask(r.ctx, id)
	if err != nil {
		return err
	}
	flags := r.flags("tasks", "update")
	name := flags.String("name", task.Name, "task name")
	projectID := flags.Int("project", task.ProjectID, "project ID")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if flags.NFlag() == 0 {
		return fmt.Errorf("expected at least one update option")
	}
	task.Name, task.ProjectID = *name, *projectID
	if err := r.stor.UpdateTask(r.ctx, task); err != nil {
		return err
	}
	return r.status("updated", "task", id)
}

func (r runner) deleteTask(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: chankat tasks delete ID")
	}
	id, err := parseID(args[0], "task")
	if err != nil {
		return err
	}
	if err := r.stor.DeleteTask(r.ctx, id); err != nil {
		return err
	}
	return r.status("deleted", "task", id)
}

func (r runner) startTask(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: chankat tasks start ID [--at TIME]")
	}
	id, err := parseID(args[0], "task")
	if err != nil {
		return err
	}
	flags := r.flags("tasks", "start")
	at := flags.String("at", "", "RFC3339 or YYYY-MM-DD HH:MM")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	startedAt := r.now()
	if changed(flags, "at") {
		startedAt, err = parseDateTime(*at)
		if err != nil {
			return err
		}
	}
	if err := r.stor.StartTask(r.ctx, id, startedAt); err != nil {
		return err
	}
	return r.status("started", "task", id)
}

func formatMinorMap(amounts map[string]int64) string {
	currencies := storage.SortedCurrencies(amounts)
	parts := make([]string, len(currencies))
	for i, currency := range currencies {
		parts[i] = fmt.Sprintf("%s:%d", currency, amounts[currency])
	}
	return strings.Join(parts, ",")
}
