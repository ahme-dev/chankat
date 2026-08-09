package cli

import (
	"fmt"
	"time"

	"chankat/internal/storage"
)

type dashboardOutput struct {
	Period         string                   `json:"period"`
	From           string                   `json:"from,omitempty"`
	To             string                   `json:"to,omitempty"`
	TrackedSeconds int64                    `json:"tracked_seconds"`
	EarnedMinor    map[string]int64         `json:"earned_minor"`
	PaidMinor      map[string]int64         `json:"paid_minor"`
	NetMinor       map[string]int64         `json:"net_minor"`
	Projects       []dashboardProjectOutput `json:"projects"`
}

type dashboardProjectOutput struct {
	ProjectID      *int                  `json:"project_id"`
	ProjectName    string                `json:"project_name"`
	TrackedSeconds int64                 `json:"tracked_seconds"`
	EarnedMinor    map[string]int64      `json:"earned_minor"`
	PaidMinor      map[string]int64      `json:"paid_minor"`
	NetMinor       map[string]int64      `json:"net_minor"`
	Tasks          []dashboardTaskOutput `json:"tasks"`
}

type dashboardTaskOutput struct {
	TaskID         *int             `json:"task_id"`
	TaskName       string           `json:"task_name"`
	TrackedSeconds int64            `json:"tracked_seconds"`
	EarnedMinor    map[string]int64 `json:"earned_minor"`
}

func (r runner) runDashboard(args []string) error {
	flags := r.flags("dashboard", "show")
	periodName := flags.String("period", "day", "day, week, month, or all")
	from := flags.String("from", "", "custom range start date (YYYY-MM-DD)")
	to := flags.String("to", "", "custom range end date (YYYY-MM-DD, inclusive)")
	projectID := flags.Int("project", 0, "filter by project ID")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}

	customFrom := changed(flags, "from")
	customTo := changed(flags, "to")
	if customFrom != customTo {
		return fmt.Errorf("--from and --to must be supplied together")
	}
	if customFrom && changed(flags, "period") {
		return fmt.Errorf("--period cannot be combined with --from and --to")
	}

	now := r.now()
	var period storage.Period
	var err error
	if customFrom {
		start, parseErr := parseDate(*from)
		if parseErr != nil {
			return fmt.Errorf("invalid --from: %w", parseErr)
		}
		end, parseErr := parseDate(*to)
		if parseErr != nil {
			return fmt.Errorf("invalid --to: %w", parseErr)
		}
		period, err = storage.CustomPeriod(start, end)
	} else {
		period, err = storage.CurrentPeriod(storage.PeriodKind(*periodName), now)
	}
	if err != nil {
		return err
	}

	projects, err := r.stor.GetProjects(r.ctx)
	if err != nil {
		return fmt.Errorf("load dashboard projects: %w", err)
	}
	if changed(flags, "project") {
		if *projectID <= 0 {
			return fmt.Errorf("invalid project ID %q", fmt.Sprint(*projectID))
		}
		project, getErr := r.stor.GetProject(r.ctx, *projectID)
		if getErr != nil {
			return getErr
		}
		projects = []storage.Project{project}
	}
	tasks, err := r.stor.GetTasks(r.ctx)
	if err != nil {
		return fmt.Errorf("load dashboard tasks: %w", err)
	}
	rates, err := r.stor.GetRates(r.ctx)
	if err != nil {
		return fmt.Errorf("load dashboard rates: %w", err)
	}
	entries, err := r.stor.GetEntries(r.ctx)
	if err != nil {
		return fmt.Errorf("load dashboard entries: %w", err)
	}
	payments, err := r.stor.GetPayments(r.ctx)
	if err != nil {
		return fmt.Errorf("load dashboard payments: %w", err)
	}
	if changed(flags, "project") {
		entries = filterDashboardEntries(entries, *projectID)
		payments = filterDashboardPayments(payments, *projectID)
	}

	summary := storage.SummarizeDashboard(
		projects, tasks, rates, entries, payments,
		period.Start, period.End, now,
	)
	output := makeDashboardOutput(period, summary)
	if r.json {
		return r.writeJSON(output)
	}
	if _, err := fmt.Fprintf(r.out, "%s\nTracked: %s\n\n", period.Label(),
		formatTracked(output.TrackedSeconds)); err != nil {
		return err
	}
	rows := make([]string, len(output.Projects)+1)
	for i, project := range output.Projects {
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s",
			project.ProjectName, formatTracked(project.TrackedSeconds),
			formatMinorMap(project.EarnedMinor), formatMinorMap(project.PaidMinor),
			formatMinorMap(project.NetMinor))
	}
	rows[len(rows)-1] = fmt.Sprintf("TOTAL\t%s\t%s\t%s\t%s",
		formatTracked(output.TrackedSeconds), formatMinorMap(output.EarnedMinor),
		formatMinorMap(output.PaidMinor), formatMinorMap(output.NetMinor))
	return r.table("PROJECT\tTRACKED\tEARNED_MINOR\tPAID_MINOR\tNET_MINOR", rows)
}

func filterDashboardEntries(entries []storage.Entry, projectID int) []storage.Entry {
	result := make([]storage.Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.ProjectID != nil && *entry.ProjectID == projectID {
			result = append(result, entry)
		}
	}
	return result
}

func filterDashboardPayments(payments []storage.Payment, projectID int) []storage.Payment {
	result := make([]storage.Payment, 0, len(payments))
	for _, payment := range payments {
		if payment.ProjectID == projectID {
			result = append(result, payment)
		}
	}
	return result
}

func makeDashboardOutput(
	period storage.Period,
	summary storage.DashboardSummary,
) dashboardOutput {
	output := dashboardOutput{
		Period: string(period.Kind), TrackedSeconds: int64(summary.Tracked / time.Second),
		EarnedMinor: summary.EarnedMinor, PaidMinor: summary.PaidMinor,
		NetMinor: summary.NetMinor,
		Projects: make([]dashboardProjectOutput, 0, len(summary.Projects)),
	}
	if period.Kind == "" {
		output.Period = "custom"
	}
	if !period.Start.IsZero() {
		output.From = period.Start.Format(dateLayout)
	}
	if !period.End.IsZero() {
		output.To = period.End.AddDate(0, 0, -1).Format(dateLayout)
	}
	for _, project := range summary.Projects {
		item := dashboardProjectOutput{
			ProjectID: project.ProjectID, ProjectName: project.ProjectName,
			TrackedSeconds: int64(project.Tracked / time.Second),
			EarnedMinor:    project.EarnedMinor, PaidMinor: project.PaidMinor,
			NetMinor: project.NetMinor,
			Tasks:    make([]dashboardTaskOutput, 0, len(project.Tasks)),
		}
		for _, task := range project.Tasks {
			item.Tasks = append(item.Tasks, dashboardTaskOutput{
				TaskID: task.TaskID, TaskName: task.TaskName,
				TrackedSeconds: int64(task.Tracked / time.Second),
				EarnedMinor:    task.EarnedMinor,
			})
		}
		output.Projects = append(output.Projects, item)
	}
	return output
}
