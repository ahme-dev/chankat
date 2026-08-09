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

type DashboardTimeBucket struct {
	Start   time.Time
	End     time.Time
	Tracked time.Duration
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

// SummarizeDashboardTimeline splits tracked time into calendar-aware buckets.
func SummarizeDashboardTimeline(
	entries []Entry,
	period Period,
	now time.Time,
) []DashboardTimeBucket {
	start, end := period.Start, period.End
	if end.IsZero() {
		end = now
	}
	if start.IsZero() {
		start = end
		for _, entry := range entries {
			if entry.StartedAt.Before(start) {
				start = entry.StartedAt
			}
		}
	}
	if !end.After(start) {
		return nil
	}

	unit := timelineBucketUnit(period.Kind, end.Sub(start))
	start = alignTimelineStart(start, unit)
	buckets := make([]DashboardTimeBucket, 0)
	for bucketStart := start; bucketStart.Before(end); {
		bucketEnd := nextTimelineStart(bucketStart, unit)
		buckets = append(buckets, DashboardTimeBucket{
			Start: bucketStart,
			End:   bucketEnd,
		})
		bucketStart = bucketEnd
	}

	for _, entry := range entries {
		entryEnd := now
		if entry.EndedAt != nil && entry.EndedAt.Before(entryEnd) {
			entryEnd = *entry.EndedAt
		}
		if entryEnd.After(end) {
			entryEnd = end
		}
		entryStart := entry.StartedAt
		if entryStart.Before(start) {
			entryStart = start
		}
		if !entryEnd.After(entryStart) {
			continue
		}
		for i := range buckets {
			overlapStart := entryStart
			if buckets[i].Start.After(overlapStart) {
				overlapStart = buckets[i].Start
			}
			overlapEnd := entryEnd
			if buckets[i].End.Before(overlapEnd) {
				overlapEnd = buckets[i].End
			}
			if overlapEnd.After(overlapStart) {
				buckets[i].Tracked += overlapEnd.Sub(overlapStart)
			}
		}
	}
	return buckets
}

type timelineUnit int

const (
	timelineHour timelineUnit = iota
	timelineDay
	timelineMonth
	timelineYear
)

func timelineBucketUnit(kind PeriodKind, span time.Duration) timelineUnit {
	switch kind {
	case Day:
		return timelineHour
	case Week, Month:
		return timelineDay
	}
	switch {
	case span <= 48*time.Hour:
		return timelineHour
	case span <= 90*24*time.Hour:
		return timelineDay
	case span <= 2*365*24*time.Hour:
		return timelineMonth
	default:
		return timelineYear
	}
}

func alignTimelineStart(value time.Time, unit timelineUnit) time.Time {
	switch unit {
	case timelineHour:
		return time.Date(
			value.Year(), value.Month(), value.Day(), value.Hour(),
			0, 0, 0, value.Location(),
		)
	case timelineDay:
		return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
	case timelineMonth:
		return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, value.Location())
	default:
		return time.Date(value.Year(), 1, 1, 0, 0, 0, 0, value.Location())
	}
}

func nextTimelineStart(value time.Time, unit timelineUnit) time.Time {
	switch unit {
	case timelineHour:
		return value.Add(time.Hour)
	case timelineDay:
		return value.AddDate(0, 0, 1)
	case timelineMonth:
		return value.AddDate(0, 1, 0)
	default:
		return value.AddDate(1, 0, 0)
	}
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
