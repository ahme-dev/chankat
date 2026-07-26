package storage

import (
	"context"
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
	`

	_, err := s.db.ExecContext(ctx, query, rate.Name, rate.AmountMinor, rate.Currency)
	if err != nil {
		return fmt.Errorf("update rate: %w", err)
	}

	return nil
}
