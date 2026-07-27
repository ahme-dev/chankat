package storage

import (
	"context"
	"database/sql"
	"fmt"
)

type Rate struct {
	ID          int    `db:"id"`
	Name        string `db:"name"`
	AmountMinor int    `db:"amount_minor"`
	Currency    string `db:"currency"`
}

func (s *Storage) GetRates(ctx context.Context) ([]Rate, error) {
	const query = `
		SELECT
			ID AS id,
			NAME AS name,
			AMOUNT_MINOR AS amount_minor,
			CURRENCY AS currency
		FROM RATE
		ORDER BY ID
	`

	rates := make([]Rate, 0)
	if err := s.db.SelectContext(ctx, &rates, query); err != nil {
		return nil, fmt.Errorf("query rates: %w", err)
	}

	return rates, nil
}

func (s *Storage) GetRate(ctx context.Context, id int) (Rate, error) {
	const query = `
		SELECT
			ID AS id,
			NAME AS name,
			AMOUNT_MINOR AS amount_minor,
			CURRENCY AS currency
		FROM RATE
		WHERE ID = $1
	`

	var rate Rate
	if err := s.db.GetContext(ctx, &rate, query, id); err != nil {
		if err == sql.ErrNoRows {
			return Rate{}, fmt.Errorf("rate %d not found", id)
		}
		return Rate{}, fmt.Errorf("get rate: %w", err)
	}
	return rate, nil
}

func (s *Storage) CreateRate(ctx context.Context, rate Rate) error {
	const query = `
		INSERT INTO RATE (NAME, AMOUNT_MINOR, CURRENCY)
		VALUES ($1, $2, $3)
	`

	_, err := s.db.ExecContext(ctx, query, rate.Name, rate.AmountMinor, rate.Currency)
	if err != nil {
		return fmt.Errorf("create rate: %w", err)
	}

	return nil
}

func (s *Storage) UpdateRate(ctx context.Context, rate Rate) error {
	const query = `
		UPDATE RATE
		SET NAME = $1, AMOUNT_MINOR = $2, CURRENCY = $3
		WHERE ID = $4
		  AND NOT EXISTS (
			SELECT 1
			FROM ENTRY
			WHERE RATE_ID = $4
		  )
	`

	result, err := s.db.ExecContext(
		ctx,
		query,
		rate.Name,
		rate.AmountMinor,
		rate.Currency,
		rate.ID,
	)
	if err != nil {
		return fmt.Errorf("update rate: %w", err)
	}

	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get updated rate count: %w", err)
	}
	if updated == 0 {
		var referenced bool
		if err := s.db.GetContext(ctx, &referenced, `
			SELECT EXISTS (
				SELECT 1
				FROM ENTRY
				WHERE RATE_ID = $1
			)
		`, rate.ID); err != nil {
			return fmt.Errorf("check rate references: %w", err)
		}
		if referenced {
			return fmt.Errorf("rate %d is referenced and cannot be updated", rate.ID)
		}
		return fmt.Errorf("rate %d not found", rate.ID)
	}

	return nil
}

func (s *Storage) DeleteRate(ctx context.Context, id int) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM RATE WHERE ID = $1`, id)
	if err != nil {
		return fmt.Errorf("delete rate: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get deleted rate count: %w", err)
	}
	if deleted == 0 {
		return fmt.Errorf("rate %d not found", id)
	}
	return nil
}
