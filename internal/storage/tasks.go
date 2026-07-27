package storage

import (
	"context"
	"database/sql"
	"fmt"
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
	const query = `
		INSERT INTO TASK (NAME, PROJECT_ID)
		VALUES ($1, $2)
	`

	if _, err := s.db.ExecContext(ctx, query, task.Name, task.ProjectID); err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (s *Storage) UpdateTask(ctx context.Context, task Task) error {
	const query = `
		UPDATE TASK
		SET NAME = $1, PROJECT_ID = $2
		WHERE ID = $3
	`

	result, err := s.db.ExecContext(ctx, query, task.Name, task.ProjectID, task.ID)
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
