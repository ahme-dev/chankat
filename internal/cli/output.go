package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"chankat/internal/storage"
)

type rateOutput struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	AmountMinor  int    `json:"amount_minor"`
	Currency     string `json:"currency"`
	ProjectCount int    `json:"project_count"`
}

type projectOutput struct {
	ID             int              `json:"id"`
	Name           string           `json:"name"`
	RateID         int              `json:"rate_id"`
	RateName       string           `json:"rate_name"`
	TrackedSeconds int64            `json:"tracked_seconds"`
	BalanceMinor   map[string]int64 `json:"balance_minor"`
}

type taskOutput struct {
	ID             int              `json:"id"`
	Name           string           `json:"name"`
	ProjectID      int              `json:"project_id"`
	ProjectName    string           `json:"project_name"`
	Active         bool             `json:"active"`
	LastEndedAt    *string          `json:"last_ended_at"`
	TrackedSeconds int64            `json:"tracked_seconds"`
	EarnedMinor    map[string]int64 `json:"earned_minor"`
}

type entryOutput struct {
	ID        int     `json:"id"`
	TaskID    *int    `json:"task_id"`
	ProjectID *int    `json:"project_id"`
	RateID    *int    `json:"rate_id"`
	StartedAt string  `json:"started_at"`
	EndedAt   *string `json:"ended_at"`
	Note      string  `json:"note"`
}

type paymentOutput struct {
	ID          int    `json:"id"`
	ProjectID   int    `json:"project_id"`
	ProjectName string `json:"project_name"`
	AmountMinor int    `json:"amount_minor"`
	Currency    string `json:"currency"`
	PaidAt      string `json:"paid_at"`
	PaidForDate string `json:"paid_for_date"`
	Note        string `json:"note"`
}

func (r runner) writeJSON(value any) error {
	encoder := json.NewEncoder(r.out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func (r runner) table(header string, rows []string) error {
	writer := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, header); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(writer, row); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func formatTracked(seconds int64) string {
	duration := time.Duration(seconds) * time.Second
	hours := duration / time.Hour
	minutes := duration % time.Hour / time.Minute
	remainingSeconds := duration % time.Minute / time.Second

	switch {
	case hours > 0 && remainingSeconds > 0:
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, remainingSeconds)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0 && remainingSeconds > 0:
		return fmt.Sprintf("%dm %ds", minutes, remainingSeconds)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return fmt.Sprintf("%ds", remainingSeconds)
	}
}

func rateOutputs(items []storage.RateSummary) []rateOutput {
	result := make([]rateOutput, len(items))
	for i, item := range items {
		result[i] = rateOutput{
			ID: item.ID, Name: item.Name, AmountMinor: item.AmountMinor,
			Currency: item.Currency, ProjectCount: item.ProjectCount,
		}
	}
	return result
}

func projectOutputs(items []storage.ProjectSummary) []projectOutput {
	result := make([]projectOutput, len(items))
	for i, item := range items {
		result[i] = projectOutput{
			ID: item.ID, Name: item.Name, RateID: item.RateID,
			RateName:       item.Rate.Name,
			TrackedSeconds: int64(item.Tracked / time.Second),
			BalanceMinor:   item.BalanceMinor,
		}
	}
	return result
}

func taskOutputs(items []storage.TaskSummary) []taskOutput {
	result := make([]taskOutput, len(items))
	for i, item := range items {
		var lastEndedAt *string
		if item.LastEndedAt != nil {
			value := item.LastEndedAt.Format(time.RFC3339)
			lastEndedAt = &value
		}
		result[i] = taskOutput{
			ID: item.ID, Name: item.Name, ProjectID: item.ProjectID,
			ProjectName: item.Project.Name, Active: item.Active,
			LastEndedAt:    lastEndedAt,
			TrackedSeconds: int64(item.Tracked / time.Second),
			EarnedMinor:    item.EarnedMinor,
		}
	}
	return result
}

func entryOutputs(entries []storage.Entry) []entryOutput {
	result := make([]entryOutput, len(entries))
	for i, entry := range entries {
		var endedAt *string
		if entry.EndedAt != nil {
			value := entry.EndedAt.Format(time.RFC3339)
			endedAt = &value
		}
		result[i] = entryOutput{
			ID: entry.ID, TaskID: entry.TaskID, ProjectID: entry.ProjectID,
			RateID: entry.RateID, StartedAt: entry.StartedAt.Format(time.RFC3339),
			EndedAt: endedAt, Note: entry.Note,
		}
	}
	return result
}

func paymentOutputs(
	payments []storage.Payment,
	projects []storage.Project,
) []paymentOutput {
	projectNames := make(map[int]string, len(projects))
	for _, project := range projects {
		projectNames[project.ID] = project.Name
	}
	result := make([]paymentOutput, len(payments))
	for i, payment := range payments {
		result[i] = paymentOutput{
			ID: payment.ID, ProjectID: payment.ProjectID,
			ProjectName: projectNames[payment.ProjectID],
			AmountMinor: payment.AmountMinor, Currency: payment.Currency,
			PaidAt:      payment.PaidAt.Format("2006-01-02"),
			PaidForDate: payment.PaidForDate.Format("2006-01-02"),
			Note:        payment.Note,
		}
	}
	return result
}
