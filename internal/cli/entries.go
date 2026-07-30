package cli

import (
	"fmt"
	"time"

	"chankat/internal/storage"
)

func (r runner) runEntries(args []string) error {
	if len(args) == 0 {
		return r.listEntries(nil)
	}
	switch args[0] {
	case "list":
		return r.listEntries(args[1:])
	case "get":
		return r.getEntry(args[1:])
	case "create":
		return r.createEntry(args[1:])
	case "update":
		return r.updateEntry(args[1:])
	case "delete":
		return r.deleteEntry(args[1:])
	case "stop":
		return r.stopEntry(args[1:])
	case "help", "-h", "--help":
		fmt.Fprintln(r.out, "Usage: chankat entries list|get|create|update|delete|stop")
		return nil
	default:
		return fmt.Errorf("unknown entries command %q", args[0])
	}
}

func (r runner) listEntries(args []string) error {
	flags := r.flags("entries list",
		"Usage: chankat entries list [--task ID] [--active]")
	taskID := flags.Int("task", 0, "filter by task ID")
	active := flags.Bool("active", false, "show only active entries")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	entries, err := r.stor.GetEntries(r.ctx)
	if err != nil {
		return fmt.Errorf("list entries: %w", err)
	}
	filtered := make([]storage.Entry, 0, len(entries))
	for _, entry := range entries {
		if changed(flags, "task") &&
			(entry.TaskID == nil || *entry.TaskID != *taskID) {
			continue
		}
		if *active && entry.EndedAt != nil {
			continue
		}
		filtered = append(filtered, entry)
	}
	output := entryOutputs(filtered)
	if r.json {
		return r.writeJSON(output)
	}
	rows := make([]string, len(output))
	for i, item := range output {
		rows[i] = fmt.Sprintf("%d\t%s\t%s\t%s\t%s\t%s\t%s", item.ID,
			pointerInt(item.TaskID), pointerInt(item.ProjectID),
			pointerInt(item.RateID), item.StartedAt, pointerString(item.EndedAt),
			item.Note)
	}
	return r.table(
		"ID\tTASK_ID\tPROJECT_ID\tRATE_ID\tSTARTED_AT\tENDED_AT\tNOTE",
		rows,
	)
}

func (r runner) getEntry(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: chankat entries get ID")
	}
	id, err := parseID(args[0], "entry")
	if err != nil {
		return err
	}
	entry, err := r.stor.GetEntry(r.ctx, id)
	if err != nil {
		return err
	}
	output := entryOutputs([]storage.Entry{entry})[0]
	if r.json {
		return r.writeJSON(output)
	}
	return r.table(
		"ID\tTASK_ID\tPROJECT_ID\tRATE_ID\tSTARTED_AT\tENDED_AT\tNOTE",
		[]string{fmt.Sprintf("%d\t%s\t%s\t%s\t%s\t%s\t%s", output.ID,
			pointerInt(output.TaskID), pointerInt(output.ProjectID),
			pointerInt(output.RateID), output.StartedAt,
			pointerString(output.EndedAt), output.Note)},
	)
}

func (r runner) createEntry(args []string) error {
	flags := r.flags("entries create", `Usage: chankat entries create --task ID --started-at TIME [options]
  --ended-at TIME
  --note TEXT`)
	taskID := flags.Int("task", 0, "task ID")
	startedAt := flags.String("started-at", "", "RFC3339 or YYYY-MM-DD HH:MM")
	endedAt := flags.String("ended-at", "", "RFC3339 or YYYY-MM-DD HH:MM")
	note := flags.String("note", "", "entry note")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if err := required(flags, "task", "started-at"); err != nil {
		return err
	}
	started, err := parseDateTime(*startedAt)
	if err != nil {
		return err
	}
	var ended *time.Time
	if changed(flags, "ended-at") {
		value, err := parseDateTime(*endedAt)
		if err != nil {
			return err
		}
		ended = &value
	}
	id, err := r.stor.CreateEntryForTaskID(
		r.ctx, *taskID, started, ended, *note,
	)
	if err != nil {
		return err
	}
	return r.status("created", "entry", id)
}

func (r runner) updateEntry(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: chankat entries update ID [options]")
	}
	id, err := parseID(args[0], "entry")
	if err != nil {
		return err
	}
	entry, err := r.stor.GetEntry(r.ctx, id)
	if err != nil {
		return err
	}
	endedDefault := ""
	if entry.EndedAt != nil {
		endedDefault = entry.EndedAt.Format(time.RFC3339)
	}
	flags := r.flags("entries update", `Usage: chankat entries update ID [options]
An empty --ended-at value makes the entry active.`)
	startedAt := flags.String("started-at", entry.StartedAt.Format(time.RFC3339),
		"RFC3339 or YYYY-MM-DD HH:MM")
	endedAt := flags.String("ended-at", endedDefault,
		"RFC3339 or YYYY-MM-DD HH:MM; empty means active")
	note := flags.String("note", entry.Note, "entry note")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if flags.NFlag() == 0 {
		return fmt.Errorf("expected at least one update option")
	}
	entry.StartedAt, err = parseDateTime(*startedAt)
	if err != nil {
		return err
	}
	if *endedAt == "" {
		entry.EndedAt = nil
	} else {
		value, err := parseDateTime(*endedAt)
		if err != nil {
			return err
		}
		entry.EndedAt = &value
	}
	entry.Note = *note
	if err := r.stor.UpdateEntry(r.ctx, entry); err != nil {
		return err
	}
	return r.status("updated", "entry", id)
}

func (r runner) deleteEntry(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: chankat entries delete ID")
	}
	id, err := parseID(args[0], "entry")
	if err != nil {
		return err
	}
	if err := r.stor.DeleteEntry(r.ctx, id); err != nil {
		return err
	}
	return r.status("deleted", "entry", id)
}

func (r runner) stopEntry(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: chankat entries stop ID | chankat entries stop --all")
	}
	endedAt := r.now()
	if args[0] == "--all" {
		count, err := r.stor.PauseAllEntries(r.ctx, endedAt)
		if err != nil {
			return err
		}
		if r.json {
			return r.writeJSON(struct {
				Status string `json:"status"`
				Entity string `json:"entity"`
				Count  int    `json:"count"`
			}{Status: "stopped", Entity: "entries", Count: count})
		}
		_, err = fmt.Fprintf(r.out, "%d entries stopped\n", count)
		return err
	}
	id, err := parseID(args[0], "entry")
	if err != nil {
		return err
	}
	if err := r.stor.PauseEntry(r.ctx, id, endedAt); err != nil {
		return err
	}
	return r.status("stopped", "entry", id)
}

func pointerInt(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(*value)
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
