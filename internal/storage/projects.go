package storage

import (
	"context"
	"database/sql"
	"fmt"
)

type Project struct {
	ID     int    `db:"id"`
	Name   string `db:"name"`
	RateID int    `db:"rate_id"`
}

func (s *Storage) GetProjects(ctx context.Context) ([]Project, error) {
	const query = `
		SELECT ID AS id, NAME AS name, RATE_ID AS rate_id
		FROM PROJECT
		ORDER BY ID
	`

	projects := make([]Project, 0)
	if err := s.db.SelectContext(ctx, &projects, query); err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	return projects, nil
}

func (s *Storage) GetProject(ctx context.Context, id int) (Project, error) {
	const query = `
		SELECT ID AS id, NAME AS name, RATE_ID AS rate_id
		FROM PROJECT
		WHERE ID = $1
	`

	var project Project
	if err := s.db.GetContext(ctx, &project, query, id); err != nil {
		if err == sql.ErrNoRows {
			return Project{}, fmt.Errorf("project %d not found", id)
		}
		return Project{}, fmt.Errorf("get project: %w", err)
	}
	return project, nil
}

func (s *Storage) CreateProject(ctx context.Context, project Project) error {
	_, err := s.CreateProjectID(ctx, project)
	return err
}

func (s *Storage) CreateProjectID(ctx context.Context, project Project) (int, error) {
	name, err := NormalizeName(project.Name, "name")
	if err != nil {
		return 0, fmt.Errorf("create project: %w", err)
	}
	const query = `
		INSERT INTO PROJECT (NAME, RATE_ID)
		VALUES ($1, $2)
	`

	result, err := s.db.ExecContext(ctx, query, name, project.RateID)
	if err != nil {
		return 0, fmt.Errorf("create project: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get created project ID: %w", err)
	}
	return int(id), nil
}

func (s *Storage) UpdateProject(ctx context.Context, project Project) error {
	name, err := NormalizeName(project.Name, "name")
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	const query = `
		UPDATE PROJECT
		SET NAME = $1, RATE_ID = $2
		WHERE ID = $3
	`

	result, err := s.db.ExecContext(ctx, query, name, project.RateID, project.ID)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get updated project count: %w", err)
	}
	if updated == 0 {
		return fmt.Errorf("project %d not found", project.ID)
	}
	return nil
}

func (s *Storage) DeleteProject(ctx context.Context, id int) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM PROJECT WHERE ID = $1`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get deleted project count: %w", err)
	}
	if deleted == 0 {
		return fmt.Errorf("project %d not found", id)
	}
	return nil
}
