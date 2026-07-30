package cli

import (
	"fmt"

	"chankat/internal/storage"
)

func (r runner) runProjects(args []string) error {
	if len(args) == 0 {
		return r.listProjects(nil)
	}
	switch args[0] {
	case "list":
		return r.listProjects(args[1:])
	case "get":
		return r.getProject(args[1:])
	case "create":
		return r.createProject(args[1:])
	case "update":
		return r.updateProject(args[1:])
	case "delete":
		return r.deleteProject(args[1:])
	case "help", "-h", "--help":
		fmt.Fprintln(r.out, "Usage: chankat projects list|get|create|update|delete")
		return nil
	default:
		return fmt.Errorf("unknown projects command %q", args[0])
	}
}

func (r runner) loadProjects() ([]storage.ProjectSummary, error) {
	projects, err := r.stor.GetProjects(r.ctx)
	if err != nil {
		return nil, err
	}
	rates, err := r.stor.GetRates(r.ctx)
	if err != nil {
		return nil, err
	}
	entries, err := r.stor.GetEntries(r.ctx)
	if err != nil {
		return nil, err
	}
	payments, err := r.stor.GetPayments(r.ctx)
	if err != nil {
		return nil, err
	}
	return storage.SummarizeProjects(projects, rates, entries, payments), nil
}

func (r runner) listProjects(args []string) error {
	if err := requireNoArgs(args, "chankat projects list"); err != nil {
		return err
	}
	items, err := r.loadProjects()
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}
	output := projectOutputs(items)
	if r.json {
		return r.writeJSON(output)
	}
	rows := make([]string, len(output))
	for i, item := range output {
		rows[i] = fmt.Sprintf("%d\t%s\t%d\t%s\t%s\t%v", item.ID, item.Name,
			item.RateID, item.RateName, formatTracked(item.TrackedSeconds),
			item.BalanceMinor)
	}
	return r.table("ID\tNAME\tRATE_ID\tRATE\tTRACKED\tBALANCE_MINOR", rows)
}

func (r runner) getProject(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: chankat projects get ID")
	}
	id, err := parseID(args[0], "project")
	if err != nil {
		return err
	}
	items, err := r.loadProjects()
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}
	for _, item := range items {
		if item.ID == id {
			output := projectOutputs([]storage.ProjectSummary{item})[0]
			if r.json {
				return r.writeJSON(output)
			}
			return r.table("ID\tNAME\tRATE_ID\tRATE\tTRACKED\tBALANCE_MINOR",
				[]string{fmt.Sprintf("%d\t%s\t%d\t%s\t%s\t%v", output.ID,
					output.Name, output.RateID, output.RateName,
					formatTracked(output.TrackedSeconds), output.BalanceMinor)})
		}
	}
	return fmt.Errorf("project %d not found", id)
}

func (r runner) createProject(args []string) error {
	flags := r.flags("projects create",
		"Usage: chankat projects create --name NAME --rate ID")
	name := flags.String("name", "", "project name")
	rateID := flags.Int("rate", 0, "rate ID")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if err := required(flags, "name", "rate"); err != nil {
		return err
	}
	id, err := r.stor.CreateProjectID(r.ctx, storage.Project{
		Name: *name, RateID: *rateID,
	})
	if err != nil {
		return err
	}
	return r.status("created", "project", id)
}

func (r runner) updateProject(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: chankat projects update ID [options]")
	}
	id, err := parseID(args[0], "project")
	if err != nil {
		return err
	}
	project, err := r.stor.GetProject(r.ctx, id)
	if err != nil {
		return err
	}
	flags := r.flags("projects update",
		"Usage: chankat projects update ID [--name NAME] [--rate ID]")
	name := flags.String("name", project.Name, "project name")
	rateID := flags.Int("rate", project.RateID, "rate ID")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if flags.NFlag() == 0 {
		return fmt.Errorf("expected at least one update option")
	}
	project.Name, project.RateID = *name, *rateID
	if err := r.stor.UpdateProject(r.ctx, project); err != nil {
		return err
	}
	return r.status("updated", "project", id)
}

func (r runner) deleteProject(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: chankat projects delete ID")
	}
	id, err := parseID(args[0], "project")
	if err != nil {
		return err
	}
	if err := r.stor.DeleteProject(r.ctx, id); err != nil {
		return err
	}
	return r.status("deleted", "project", id)
}
