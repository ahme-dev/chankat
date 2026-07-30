package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Task struct {
	ID        int    `db:"id"`
	Name      string `db:"name"`
	ProjectID int    `db:"project_id"`
}

func (s *Storage) GetTasks(ctx context.Context) ([]Task, error) {
	const query = `
		SELECT ID AS id, NAME AS name, PROJECT_ID AS project_id
		FROM TASK
		ORDER BY ID
	`

	tasks := make([]Task, 0)
	if err := s.db.SelectContext(ctx, &tasks, query); err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	return tasks, nil
}

func (s *Storage) GetTask(ctx context.Context, id int) (Task, error) {
	const query = `
		SELECT ID AS id, NAME AS name, PROJECT_ID AS project_id
		FROM TASK
		WHERE ID = $1
	`

	var task Task
	if err := s.db.GetContext(ctx, &task, query, id); err != nil {
		if err == sql.ErrNoRows {
			return Task{}, fmt.Errorf("task %d not found", id)
		}
		return Task{}, fmt.Errorf("get task: %w", err)
	}
	return task, nil
}

func (s *Storage) CreateTask(ctx context.Context, task Task) error {
	_, err := s.CreateTaskID(ctx, task)
	return err
}

func (s *Storage) CreateTaskID(ctx context.Context, task Task) (int, error) {
	name, err := NormalizeName(task.Name, "name")
	if err != nil {
		return 0, fmt.Errorf("create task: %w", err)
	}
	const query = `
		INSERT INTO TASK (NAME, PROJECT_ID)
		VALUES ($1, $2)
	`

	result, err := s.db.ExecContext(ctx, query, name, task.ProjectID)
	if err != nil {
		return 0, fmt.Errorf("create task: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get created task ID: %w", err)
	}
	return int(id), nil
}

func (s *Storage) CreateTaskAndStart(
	ctx context.Context,
	task Task,
	startedAt time.Time,
) error {
	return s.CreateTaskAndEntry(ctx, task, Entry{StartedAt: startedAt})
}

func (s *Storage) CreateTaskAndEntry(
	ctx context.Context,
	task Task,
	entry Entry,
) error {
	_, err := s.CreateTaskAndEntryID(ctx, task, entry)
	return err
}

func (s *Storage) CreateTaskAndEntryID(
	ctx context.Context,
	task Task,
	entry Entry,
) (int, error) {
	name, err := NormalizeName(task.Name, "name")
	if err != nil {
		return 0, fmt.Errorf("create task entry: %w", err)
	}
	if err := ValidateEntryTimes(entry.StartedAt, entry.EndedAt); err != nil {
		return 0, fmt.Errorf("create task entry: %w", err)
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin task entry: %w", err)
	}
	defer tx.Rollback()

	var rateID int
	if err := tx.GetContext(
		ctx,
		&rateID,
		`SELECT RATE_ID FROM PROJECT WHERE ID = $1`,
		task.ProjectID,
	); err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("project %d not found", task.ProjectID)
		}
		return 0, fmt.Errorf("get project rate: %w", err)
	}

	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO TASK (NAME, PROJECT_ID) VALUES ($1, $2)`,
		name,
		task.ProjectID,
	)
	if err != nil {
		return 0, fmt.Errorf("create task: %w", err)
	}
	taskID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get created task ID: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO ENTRY (
			TASK_ID, PROJECT_ID, RATE_ID, STARTED_AT, ENDED_AT, NOTES
		 )
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		taskID,
		task.ProjectID,
		rateID,
		entry.StartedAt.Unix(),
		unixTime(entry.EndedAt),
		strings.TrimSpace(entry.Note),
	); err != nil {
		return 0, fmt.Errorf("create task entry: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit task start: %w", err)
	}
	return int(taskID), nil
}

func (s *Storage) CreateEntryForTask(
	ctx context.Context,
	taskID int,
	startedAt time.Time,
	endedAt *time.Time,
	note string,
) error {
	_, err := s.CreateEntryForTaskID(ctx, taskID, startedAt, endedAt, note)
	return err
}

func (s *Storage) CreateEntryForTaskID(
	ctx context.Context,
	taskID int,
	startedAt time.Time,
	endedAt *time.Time,
	note string,
) (int, error) {
	if err := ValidateEntryTimes(startedAt, endedAt); err != nil {
		return 0, fmt.Errorf("create task entry: %w", err)
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin task entry: %w", err)
	}
	defer tx.Rollback()

	var task struct {
		ProjectID int `db:"project_id"`
		RateID    int `db:"rate_id"`
	}
	if err := tx.GetContext(ctx, &task, `
		SELECT TASK.PROJECT_ID AS project_id, PROJECT.RATE_ID AS rate_id
		FROM TASK
		JOIN PROJECT ON PROJECT.ID = TASK.PROJECT_ID
		WHERE TASK.ID = $1
	`, taskID); err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("task %d not found", taskID)
		}
		return 0, fmt.Errorf("get task project and rate: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO ENTRY (
			TASK_ID, PROJECT_ID, RATE_ID, STARTED_AT, ENDED_AT, NOTES
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, taskID, task.ProjectID, task.RateID, startedAt.Unix(), unixTime(endedAt),
		strings.TrimSpace(note))
	if err != nil {
		return 0, fmt.Errorf("create task entry: %w", err)
	}
	entryID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get created entry ID: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit task entry: %w", err)
	}
	return int(entryID), nil
}

func (s *Storage) StartTask(ctx context.Context, taskID int, startedAt time.Time) error {
	return s.CreateEntryForTask(ctx, taskID, startedAt, nil, "")
}

func (s *Storage) PauseTask(
	ctx context.Context,
	taskID int,
	endedAt time.Time,
) error {
	if err := validateEndTime(endedAt); err != nil {
		return fmt.Errorf("pause task: %w", err)
	}
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE ENTRY
		 SET ENDED_AT = $1
		 WHERE TASK_ID = $2
		   AND ENDED_AT IS NULL
		   AND STARTED_AT < $1`,
		endedAt.Unix(),
		taskID,
	)
	if err != nil {
		return fmt.Errorf("pause task: %w", err)
	}
	paused, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get paused entry count: %w", err)
	}
	if paused == 0 {
		return fmt.Errorf("task %d is not active", taskID)
	}
	return nil
}

func (s *Storage) PauseAllTasks(
	ctx context.Context,
	endedAt time.Time,
) (int, error) {
	if err := validateEndTime(endedAt); err != nil {
		return 0, fmt.Errorf("pause tasks: %w", err)
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin pause tasks: %w", err)
	}
	defer tx.Rollback()

	var taskIDs []int
	if err := tx.SelectContext(ctx, &taskIDs, `
		SELECT DISTINCT TASK_ID
		FROM ENTRY
		WHERE TASK_ID IS NOT NULL
		  AND ENDED_AT IS NULL
		  AND STARTED_AT < $1
	`, endedAt.Unix()); err != nil {
		return 0, fmt.Errorf("get active tasks: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE ENTRY
		SET ENDED_AT = $1
		WHERE TASK_ID IS NOT NULL
		  AND ENDED_AT IS NULL
		  AND STARTED_AT < $1
	`, endedAt.Unix()); err != nil {
		return 0, fmt.Errorf("pause tasks: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit pause tasks: %w", err)
	}
	return len(taskIDs), nil
}

func (s *Storage) UpdateTask(ctx context.Context, task Task) error {
	name, err := NormalizeName(task.Name, "name")
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	const query = `
		UPDATE TASK
		SET NAME = $1, PROJECT_ID = $2
		WHERE ID = $3
	`

	result, err := s.db.ExecContext(ctx, query, name, task.ProjectID, task.ID)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get updated task count: %w", err)
	}
	if updated == 0 {
		return fmt.Errorf("task %d not found", task.ID)
	}
	return nil
}

func (s *Storage) DeleteTask(ctx context.Context, id int) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM TASK WHERE ID = $1`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get deleted task count: %w", err)
	}
	if deleted == 0 {
		return fmt.Errorf("task %d not found", id)
	}
	return nil
}

func (s *Storage) AssignTasksToProject(
	ctx context.Context,
	taskIDs []int,
	projectID int,
) error {
	return s.bulkAssign(ctx, "TASK", "PROJECT_ID", projectID, taskIDs)
}
