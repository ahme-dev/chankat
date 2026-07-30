package storage

import (
	"sort"
	"time"
)

type RateSummary struct {
	Rate
	ProjectCount int
}

type ProjectSummary struct {
	Project
	Rate         Rate
	BalanceMinor map[string]int64
	Tracked      time.Duration
}

type TaskSummary struct {
	Task
	Project     Project
	Active      bool
	LastEndedAt *time.Time
	LastEntryID int
	Tracked     time.Duration
	EarnedMinor map[string]int64
}

func SummarizeRates(rates []Rate, projects []Project) []RateSummary {
	counts := make(map[int]int, len(rates))
	for _, project := range projects {
		counts[project.RateID]++
	}
	result := make([]RateSummary, len(rates))
	for i, rate := range rates {
		result[i] = RateSummary{Rate: rate, ProjectCount: counts[rate.ID]}
	}
	return result
}

func SummarizeProjects(
	projects []Project,
	rates []Rate,
	entries []Entry,
	payments []Payment,
) []ProjectSummary {
	ratesByID := RatesByID(rates)
	balances := make(map[int]map[string]int64, len(projects))
	minorSeconds := make(map[int]map[string]int64, len(projects))
	tracked := make(map[int]time.Duration, len(projects))
	for _, project := range projects {
		balances[project.ID] = make(map[string]int64)
		if rate, ok := ratesByID[project.RateID]; ok {
			balances[project.ID][rate.Currency] = 0
		}
		minorSeconds[project.ID] = make(map[string]int64)
	}
	for _, entry := range entries {
		if entry.ProjectID == nil || entry.RateID == nil || entry.EndedAt == nil {
			continue
		}
		rate, rateOK := ratesByID[*entry.RateID]
		_, projectOK := balances[*entry.ProjectID]
		if !rateOK || !projectOK {
			continue
		}
		elapsed := nonNegativeDuration(entry.StartedAt, *entry.EndedAt)
		tracked[*entry.ProjectID] += elapsed
		minorSeconds[*entry.ProjectID][rate.Currency] +=
			int64(rate.AmountMinor) * int64(elapsed/time.Second)
	}
	for projectID, currencies := range minorSeconds {
		for currency, total := range currencies {
			balances[projectID][currency] += total / 3600
		}
	}
	for _, payment := range payments {
		if balances[payment.ProjectID] != nil {
			balances[payment.ProjectID][payment.Currency] -= int64(payment.AmountMinor)
		}
	}

	result := make([]ProjectSummary, len(projects))
	for i, project := range projects {
		result[i] = ProjectSummary{
			Project:      project,
			Rate:         ratesByID[project.RateID],
			BalanceMinor: balances[project.ID],
			Tracked:      tracked[project.ID],
		}
	}
	return result
}

func SummarizeTasks(
	tasks []Task,
	projects []Project,
	entries []Entry,
	rates []Rate,
	now time.Time,
) []TaskSummary {
	projectsByID := make(map[int]Project, len(projects))
	for _, project := range projects {
		projectsByID[project.ID] = project
	}
	ratesByID := RatesByID(rates)
	result := make([]TaskSummary, len(tasks))
	for i, task := range tasks {
		tracked, earned := TaskTotals(entries, ratesByID, task.ID, now)
		var lastEndedAt *time.Time
		lastEntryID := 0
		active := false
		for _, entry := range entries {
			if entry.TaskID == nil || *entry.TaskID != task.ID {
				continue
			}
			if entry.EndedAt == nil {
				active = true
				continue
			}
			if lastEndedAt == nil ||
				entry.EndedAt.After(*lastEndedAt) ||
				entry.EndedAt.Equal(*lastEndedAt) && entry.ID > lastEntryID {
				value := *entry.EndedAt
				lastEndedAt = &value
				lastEntryID = entry.ID
			}
		}
		result[i] = TaskSummary{
			Task: task, Project: projectsByID[task.ProjectID], Active: active,
			LastEndedAt: lastEndedAt, LastEntryID: lastEntryID,
			Tracked: tracked, EarnedMinor: earned,
		}
	}
	return result
}

func TaskTotals(
	entries []Entry,
	rates map[int]Rate,
	taskID int,
	now time.Time,
) (time.Duration, map[string]int64) {
	var duration time.Duration
	minorSeconds := make(map[string]int64)
	for _, entry := range entries {
		if entry.TaskID == nil || *entry.TaskID != taskID {
			continue
		}
		endedAt := now
		if entry.EndedAt != nil {
			endedAt = *entry.EndedAt
		} else if now.IsZero() {
			continue
		}
		elapsed := nonNegativeDuration(entry.StartedAt, endedAt)
		duration += elapsed
		if entry.RateID == nil {
			continue
		}
		rate, ok := rates[*entry.RateID]
		if !ok {
			continue
		}
		minorSeconds[rate.Currency] +=
			int64(rate.AmountMinor) * int64(elapsed/time.Second)
	}
	amounts := make(map[string]int64, len(minorSeconds))
	for currency, total := range minorSeconds {
		amounts[currency] = total / 3600
	}
	return duration, amounts
}

func RatesByID(rates []Rate) map[int]Rate {
	result := make(map[int]Rate, len(rates))
	for _, rate := range rates {
		result[rate.ID] = rate
	}
	return result
}

func SortedCurrencies(amounts map[string]int64) []string {
	result := make([]string, 0, len(amounts))
	for currency := range amounts {
		result = append(result, currency)
	}
	sort.Strings(result)
	return result
}

func nonNegativeDuration(startedAt, endedAt time.Time) time.Duration {
	elapsed := endedAt.Sub(startedAt)
	if elapsed < 0 {
		return 0
	}
	return elapsed
}
