package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Entry struct {
	ID        int
	TaskID    *int
	ProjectID *int
	RateID    *int
	StartedAt time.Time
	EndedAt   *time.Time
	Note      string
}

type entryRow struct {
	ID        int    `db:"id"`
	TaskID    *int   `db:"task_id"`
	ProjectID *int   `db:"project_id"`
	RateID    *int   `db:"rate_id"`
	StartedAt int64  `db:"started_at"`
	EndedAt   *int64 `db:"ended_at"`
	Note      string `db:"note"`
}

const selectEntries = `
	SELECT
		ID AS id,
		TASK_ID AS task_id,
		PROJECT_ID AS project_id,
		RATE_ID AS rate_id,
		STARTED_AT AS started_at,
		ENDED_AT AS ended_at,
		NOTES AS note
	FROM ENTRY
`

func (s *Storage) GetEntries(ctx context.Context) ([]Entry, error) {
	var rows []entryRow
	if err := s.db.SelectContext(ctx, &rows, selectEntries+` ORDER BY ID`); err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}
	return entriesFromRows(rows), nil
}

func (s *Storage) GetActiveEntries(ctx context.Context) ([]Entry, error) {
	var rows []entryRow
	if err := s.db.SelectContext(
		ctx,
		&rows,
		selectEntries+` WHERE ENDED_AT IS NULL ORDER BY ID`,
	); err != nil {
		return nil, fmt.Errorf("query active entries: %w", err)
	}
	return entriesFromRows(rows), nil
}

func (s *Storage) GetEntry(ctx context.Context, id int) (Entry, error) {
	var row entryRow
	if err := s.db.GetContext(ctx, &row, selectEntries+` WHERE ID = $1`, id); err != nil {
		if err == sql.ErrNoRows {
			return Entry{}, fmt.Errorf("entry %d not found", id)
		}
		return Entry{}, fmt.Errorf("get entry: %w", err)
	}
	return entryFromRow(row), nil
}

func (s *Storage) CreateEntry(ctx context.Context, entry Entry) error {
	const query = `
		INSERT INTO ENTRY (
			TASK_ID, PROJECT_ID, RATE_ID, STARTED_AT, ENDED_AT, NOTES
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	if _, err := s.db.ExecContext(
		ctx,
		query,
		entry.TaskID,
		entry.ProjectID,
		entry.RateID,
		entry.StartedAt.Unix(),
		unixTime(entry.EndedAt),
		entry.Note,
	); err != nil {
		return fmt.Errorf("create entry: %w", err)
	}
	return nil
}

func (s *Storage) UpdateEntry(ctx context.Context, entry Entry) error {
	const query = `
		UPDATE ENTRY
		SET
			TASK_ID = $1,
			PROJECT_ID = $2,
			RATE_ID = $3,
			STARTED_AT = $4,
			ENDED_AT = $5,
			NOTES = $6
		WHERE ID = $7
	`

	result, err := s.db.ExecContext(
		ctx,
		query,
		entry.TaskID,
		entry.ProjectID,
		entry.RateID,
		entry.StartedAt.Unix(),
		unixTime(entry.EndedAt),
		entry.Note,
		entry.ID,
	)
	if err != nil {
		return fmt.Errorf("update entry: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get updated entry count: %w", err)
	}
	if updated == 0 {
		return fmt.Errorf("entry %d not found", entry.ID)
	}
	return nil
}

func (s *Storage) DeleteEntry(ctx context.Context, id int) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM ENTRY WHERE ID = $1`, id)
	if err != nil {
		return fmt.Errorf("delete entry: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get deleted entry count: %w", err)
	}
	if deleted == 0 {
		return fmt.Errorf("entry %d not found", id)
	}
	return nil
}

func (s *Storage) AssignEntriesToProject(
	ctx context.Context,
	entryIDs []int,
	projectID *int,
) error {
	return s.bulkAssign(ctx, "ENTRY", "PROJECT_ID", projectID, entryIDs)
}

func (s *Storage) AssignEntriesToRate(
	ctx context.Context,
	entryIDs []int,
	rateID *int,
) error {
	return s.bulkAssign(ctx, "ENTRY", "RATE_ID", rateID, entryIDs)
}

func entriesFromRows(rows []entryRow) []Entry {
	entries := make([]Entry, len(rows))
	for i, row := range rows {
		entries[i] = entryFromRow(row)
	}
	return entries
}

func entryFromRow(row entryRow) Entry {
	return Entry{
		ID:        row.ID,
		TaskID:    row.TaskID,
		ProjectID: row.ProjectID,
		RateID:    row.RateID,
		StartedAt: time.Unix(row.StartedAt, 0),
		EndedAt:   timeFromUnix(row.EndedAt),
		Note:      row.Note,
	}
}

func unixTime(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	unix := value.Unix()
	return &unix
}

func timeFromUnix(value *int64) *time.Time {
	if value == nil {
		return nil
	}
	timestamp := time.Unix(*value, 0)
	return &timestamp
}
