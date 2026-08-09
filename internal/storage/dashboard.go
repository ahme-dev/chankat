package storage

import (
	"sort"
	"time"
)

type DashboardSummary struct {
	Tracked     time.Duration
	EarnedMinor map[string]int64
	PaidMinor   map[string]int64
	NetMinor    map[string]int64
	Projects    []DashboardProjectSummary
}

type DashboardProjectSummary struct {
	ProjectID   *int
	ProjectName string
	Tracked     time.Duration
	EarnedMinor map[string]int64
	PaidMinor   map[string]int64
	NetMinor    map[string]int64
	Tasks       []DashboardTaskSummary
}

type DashboardTaskSummary struct {
	TaskID      *int
	TaskName    string
	Tracked     time.Duration
	EarnedMinor map[string]int64
}

type dashboardTaskTotals struct {
	tracked      time.Duration
	minorSeconds map[string]int64
}

type dashboardProjectTotals struct {
	name         string
	tracked      time.Duration
	minorSeconds map[string]int64
	paid         map[string]int64
	tasks        map[int]*dashboardTaskTotals
}

// SummarizeDashboard reports activity in the half-open interval [start, end).
// A zero start means there is no lower bound. Active entries stop at now.
func SummarizeDashboard(
	projects []Project,
	tasks []Task,
	rates []Rate,
	entries []Entry,
	payments []Payment,
	start time.Time,
	end time.Time,
	now time.Time,
) DashboardSummary {
	projectNames := make(map[int]string, len(projects))
	for _, project := range projects {
		projectNames[project.ID] = project.Name
	}
	taskNames := make(map[int]string, len(tasks))
	for _, task := range tasks {
		taskNames[task.ID] = task.Name
	}
	ratesByID := RatesByID(rates)
	totals := make(map[int]*dashboardProjectTotals)

	projectTotals := func(projectID int) *dashboardProjectTotals {
		if item, ok := totals[projectID]; ok {
			return item
		}
		name := projectNames[projectID]
		if projectID == 0 {
			name = "Unassigned"
		} else if name == "" {
			name = "Unknown project"
		}
		item := &dashboardProjectTotals{
			name: name, minorSeconds: make(map[string]int64),
			paid: make(map[string]int64), tasks: make(map[int]*dashboardTaskTotals),
		}
		totals[projectID] = item
		return item
	}

	for _, entry := range entries {
		entryEnd := now
		if entry.EndedAt != nil && entry.EndedAt.Before(entryEnd) {
			entryEnd = *entry.EndedAt
		}
		if !end.IsZero() && end.Before(entryEnd) {
			entryEnd = end
		}
		entryStart := entry.StartedAt
		if !start.IsZero() && start.After(entryStart) {
			entryStart = start
		}
		if !entryEnd.After(entryStart) {
			continue
		}

		projectID := 0
		if entry.ProjectID != nil {
			projectID = *entry.ProjectID
		}
		taskID := 0
		if entry.TaskID != nil {
			taskID = *entry.TaskID
		}
		elapsed := entryEnd.Sub(entryStart)
		project := projectTotals(projectID)
		project.tracked += elapsed
		task := project.tasks[taskID]
		if task == nil {
			task = &dashboardTaskTotals{minorSeconds: make(map[string]int64)}
			project.tasks[taskID] = task
		}
		task.tracked += elapsed

		if entry.RateID == nil {
			continue
		}
		rate, ok := ratesByID[*entry.RateID]
		if !ok {
			continue
		}
		minorSeconds := int64(rate.AmountMinor) * int64(elapsed/time.Second)
		project.minorSeconds[rate.Currency] += minorSeconds
		task.minorSeconds[rate.Currency] += minorSeconds
	}

	for _, payment := range payments {
		if !start.IsZero() && payment.PaidForDate.Before(start) {
			continue
		}
		if !end.IsZero() && !payment.PaidForDate.Before(end) {
			continue
		}
		projectTotals(payment.ProjectID).paid[payment.Currency] +=
			int64(payment.AmountMinor)
	}

	result := DashboardSummary{
		EarnedMinor: make(map[string]int64),
		PaidMinor:   make(map[string]int64),
		NetMinor:    make(map[string]int64),
	}
	for projectID, totals := range totals {
		project := DashboardProjectSummary{
			ProjectName: totals.name,
			Tracked:     totals.tracked,
			EarnedMinor: minorSecondsToAmounts(totals.minorSeconds),
			PaidMinor:   cloneAmounts(totals.paid),
		}
		if projectID != 0 {
			id := projectID
			project.ProjectID = &id
		}
		project.NetMinor = subtractAmounts(project.EarnedMinor, project.PaidMinor)

		for taskID, totals := range totals.tasks {
			taskName := taskNames[taskID]
			if taskID == 0 {
				taskName = "Unassigned"
			} else if taskName == "" {
				taskName = "Unknown task"
			}
			task := DashboardTaskSummary{
				TaskName: taskName, Tracked: totals.tracked,
				EarnedMinor: minorSecondsToAmounts(totals.minorSeconds),
			}
			if taskID != 0 {
				id := taskID
				task.TaskID = &id
			}
			project.Tasks = append(project.Tasks, task)
		}
		sort.Slice(project.Tasks, func(i, j int) bool {
			if project.Tasks[i].Tracked != project.Tasks[j].Tracked {
				return project.Tasks[i].Tracked > project.Tasks[j].Tracked
			}
			return project.Tasks[i].TaskName < project.Tasks[j].TaskName
		})

		result.Tracked += project.Tracked
		addAmounts(result.EarnedMinor, project.EarnedMinor)
		addAmounts(result.PaidMinor, project.PaidMinor)
		result.Projects = append(result.Projects, project)
	}
	result.NetMinor = subtractAmounts(result.EarnedMinor, result.PaidMinor)
	sort.Slice(result.Projects, func(i, j int) bool {
		if result.Projects[i].Tracked != result.Projects[j].Tracked {
			return result.Projects[i].Tracked > result.Projects[j].Tracked
		}
		return result.Projects[i].ProjectName < result.Projects[j].ProjectName
	})
	return result
}

func minorSecondsToAmounts(values map[string]int64) map[string]int64 {
	result := make(map[string]int64, len(values))
	for currency, value := range values {
		result[currency] = value / 3600
	}
	return result
}

func cloneAmounts(values map[string]int64) map[string]int64 {
	result := make(map[string]int64, len(values))
	addAmounts(result, values)
	return result
}

func addAmounts(target, values map[string]int64) {
	for currency, value := range values {
		target[currency] += value
	}
}

func subtractAmounts(left, right map[string]int64) map[string]int64 {
	result := cloneAmounts(left)
	for currency, value := range right {
		result[currency] -= value
	}
	return result
}
