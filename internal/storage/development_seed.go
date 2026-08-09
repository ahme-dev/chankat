package storage

import (
	"context"
	"fmt"
	"time"
)

// SeedDevelopment replaces the database contents with representative data.
// It is intended for the development seed command, not application startup.
func (s *Storage) SeedDevelopment(
	ctx context.Context,
	now time.Time,
	reset bool,
) error {
	if now.IsZero() {
		return fmt.Errorf("seed development data: current time is required")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("seed development data: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var records int
	if err := tx.GetContext(ctx, &records, `
		SELECT
			(SELECT COUNT(*) FROM RATE) +
			(SELECT COUNT(*) FROM PROJECT) +
			(SELECT COUNT(*) FROM TASK) +
			(SELECT COUNT(*) FROM ENTRY) +
			(SELECT COUNT(*) FROM PAYMENT)
	`); err != nil {
		return fmt.Errorf("seed development data: count records: %w", err)
	}
	if records > 0 && !reset {
		return fmt.Errorf(
			"seed development data: database is not empty; pass --reset to replace it",
		)
	}

	if reset {
		for _, table := range []string{"PAYMENT", "ENTRY", "TASK", "PROJECT", "RATE"} {
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
				return fmt.Errorf("seed development data: clear %s: %w", table, err)
			}
		}
	}

	for _, rate := range []Rate{
		{ID: 1, Name: "Consulting", AmountMinor: 12_500, Currency: "USD"},
		{ID: 2, Name: "Retainer", AmountMinor: 9_000, Currency: "USD"},
		{ID: 3, Name: "European", AmountMinor: 11_000, Currency: "EUR"},
	} {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO RATE (ID, NAME, AMOUNT_MINOR, CURRENCY)
			VALUES ($1, $2, $3, $4)
		`, rate.ID, rate.Name, rate.AmountMinor, rate.Currency); err != nil {
			return fmt.Errorf("seed development data: insert rate: %w", err)
		}
	}

	for _, project := range []Project{
		{ID: 1, Name: "Acme Website", RateID: 1},
		{ID: 2, Name: "Northstar API", RateID: 2},
		{ID: 3, Name: "Atelier Brand", RateID: 3},
	} {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO PROJECT (ID, NAME, RATE_ID) VALUES ($1, $2, $3)
		`, project.ID, project.Name, project.RateID); err != nil {
			return fmt.Errorf("seed development data: insert project: %w", err)
		}
	}

	for _, task := range []Task{
		{ID: 1, Name: "Landing page", ProjectID: 1},
		{ID: 2, Name: "Checkout integration", ProjectID: 1},
		{ID: 3, Name: "Authentication", ProjectID: 2},
		{ID: 4, Name: "API documentation", ProjectID: 2},
		{ID: 5, Name: "Visual identity", ProjectID: 3},
		{ID: 6, Name: "Brand guidelines", ProjectID: 3},
	} {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO TASK (ID, NAME, PROJECT_ID) VALUES ($1, $2, $3)
		`, task.ID, task.Name, task.ProjectID); err != nil {
			return fmt.Errorf("seed development data: insert task: %w", err)
		}
	}

	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	daysSinceMonday := (int(day.Weekday()) + 6) % 7
	week := day.AddDate(0, 0, -daysSinceMonday)
	month := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location())
	entryTimes := []struct {
		id, taskID, projectID, rateID int
		start, end                    time.Time
		note                          string
		active                        bool
	}{
		{1, 3, 2, 2, now.Add(-42 * time.Minute), time.Time{}, "Investigating token refresh", true},
		{2, 1, 1, 1, now.Add(-3 * time.Hour), now.Add(-time.Hour), "Responsive hero and navigation", false},
		{3, 5, 3, 3, day.AddDate(0, 0, -1).Add(13 * time.Hour), day.AddDate(0, 0, -1).Add(16 * time.Hour), "Logo directions", false},
		{4, 2, 1, 1, week.Add(9 * time.Hour), week.Add(12*time.Hour + 30*time.Minute), "Payment provider webhooks", false},
		{5, 4, 2, 2, week.AddDate(0, 0, 1).Add(10 * time.Hour), week.AddDate(0, 0, 1).Add(12 * time.Hour), "Quick-start examples", false},
		{6, 6, 3, 3, week.AddDate(0, 0, -3).Add(9 * time.Hour), week.AddDate(0, 0, -3).Add(14*time.Hour + 15*time.Minute), "Typography and color system", false},
		{7, 1, 1, 1, month.AddDate(0, -1, 5).Add(9 * time.Hour), month.AddDate(0, -1, 5).Add(12 * time.Hour), "Initial wireframes", false},
		{8, 3, 2, 2, month.AddDate(0, -1, 12).Add(13 * time.Hour), month.AddDate(0, -1, 12).Add(17 * time.Hour), "Session middleware", false},
	}
	for _, entry := range entryTimes {
		var endedAt any
		if !entry.active {
			endedAt = entry.end.Unix()
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ENTRY (
				ID, TASK_ID, PROJECT_ID, RATE_ID, STARTED_AT, ENDED_AT, NOTES
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, entry.id, entry.taskID, entry.projectID, entry.rateID,
			entry.start.Unix(), endedAt, entry.note); err != nil {
			return fmt.Errorf("seed development data: insert entry: %w", err)
		}
	}

	payments := []Payment{
		{ID: 1, ProjectID: 1, AmountMinor: 45_000, Currency: "USD", PaidAt: day, PaidForDate: week, Note: "Weekly invoice"},
		{ID: 2, ProjectID: 2, AmountMinor: 72_000, Currency: "USD", PaidAt: day.AddDate(0, 0, -2), PaidForDate: month, Note: "Retainer installment"},
		{ID: 3, ProjectID: 3, AmountMinor: 33_000, Currency: "EUR", PaidAt: month.AddDate(0, 0, -1), PaidForDate: month.AddDate(0, -1, 1), Note: "Brand discovery"},
	}
	for _, payment := range payments {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO PAYMENT (
				ID, PROJECT_ID, AMOUNT_MINOR, CURRENCY,
				PAID_AT, PAID_FOR_DATE, NOTES
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, payment.ID, payment.ProjectID, payment.AmountMinor, payment.Currency,
			payment.PaidAt.Unix(), payment.PaidForDate.Unix(), payment.Note); err != nil {
			return fmt.Errorf("seed development data: insert payment: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("seed development data: commit: %w", err)
	}
	return nil
}
