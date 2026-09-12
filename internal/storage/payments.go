package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Payment struct {
	ID          int
	ProjectID   int
	AmountMinor int
	Currency    string
	PaidAt      time.Time
	Note        string
}

type paymentRow struct {
	ID          int    `db:"id"`
	ProjectID   int    `db:"project_id"`
	AmountMinor int    `db:"amount_minor"`
	Currency    string `db:"currency"`
	PaidAt      int64  `db:"paid_at"`
	Note        string `db:"note"`
}

const selectPayments = `
	SELECT
		ID AS id,
		PROJECT_ID AS project_id,
		AMOUNT_MINOR AS amount_minor,
		CURRENCY AS currency,
		PAID_AT AS paid_at,
		NOTES AS note
	FROM PAYMENT
`

func (s *Storage) GetPayments(ctx context.Context) ([]Payment, error) {
	var rows []paymentRow
	if err := s.db.SelectContext(ctx, &rows, selectPayments+` ORDER BY ID`); err != nil {
		return nil, fmt.Errorf("query payments: %w", err)
	}
	return paymentsFromRows(rows), nil
}

func (s *Storage) GetPayment(ctx context.Context, id int) (Payment, error) {
	var row paymentRow
	if err := s.db.GetContext(ctx, &row, selectPayments+` WHERE ID = $1`, id); err != nil {
		if err == sql.ErrNoRows {
			return Payment{}, fmt.Errorf("payment %d not found", id)
		}
		return Payment{}, fmt.Errorf("get payment: %w", err)
	}
	return paymentFromRow(row), nil
}

func (s *Storage) CreatePayment(ctx context.Context, payment Payment) error {
	_, err := s.CreatePaymentID(ctx, payment)
	return err
}

func (s *Storage) CreatePaymentID(ctx context.Context, payment Payment) (int, error) {
	if err := validatePayment(payment); err != nil {
		return 0, fmt.Errorf("create payment: %w", err)
	}
	currency, err := NormalizeCurrency(payment.Currency)
	if err != nil {
		return 0, fmt.Errorf("create payment: %w", err)
	}
	if err := s.validatePaymentCurrency(ctx, payment.ProjectID, currency); err != nil {
		return 0, fmt.Errorf("create payment: %w", err)
	}
	paidAt := canonicalPaymentDate(payment.PaidAt)

	const query = `
		INSERT INTO PAYMENT (
			PROJECT_ID,
			AMOUNT_MINOR,
			CURRENCY,
			PAID_AT,
			PAID_FOR_DATE,
			NOTES
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	result, err := s.db.ExecContext(
		ctx,
		query,
		payment.ProjectID,
		payment.AmountMinor,
		currency,
		paidAt.Unix(),
		paidAt.Unix(),
		strings.TrimSpace(payment.Note),
	)
	if err != nil {
		return 0, fmt.Errorf("create payment: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get created payment ID: %w", err)
	}
	return int(id), nil
}

func (s *Storage) UpdatePayment(ctx context.Context, payment Payment) error {
	if err := validatePayment(payment); err != nil {
		return fmt.Errorf("update payment: %w", err)
	}
	currency, err := NormalizeCurrency(payment.Currency)
	if err != nil {
		return fmt.Errorf("update payment: %w", err)
	}
	existing, err := s.GetPayment(ctx, payment.ID)
	if err != nil {
		return fmt.Errorf("update payment: %w", err)
	}
	if existing.ProjectID != payment.ProjectID || existing.Currency != currency {
		if err := s.validatePaymentCurrency(ctx, payment.ProjectID, currency); err != nil {
			return fmt.Errorf("update payment: %w", err)
		}
	}
	paidAt := canonicalPaymentDate(payment.PaidAt)

	const query = `
		UPDATE PAYMENT
		SET
			PROJECT_ID = $1,
			AMOUNT_MINOR = $2,
			CURRENCY = $3,
			PAID_AT = $4,
			PAID_FOR_DATE = $5,
			NOTES = $6
		WHERE ID = $7
	`

	result, err := s.db.ExecContext(
		ctx,
		query,
		payment.ProjectID,
		payment.AmountMinor,
		currency,
		paidAt.Unix(),
		paidAt.Unix(),
		strings.TrimSpace(payment.Note),
		payment.ID,
	)
	if err != nil {
		return fmt.Errorf("update payment: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get updated payment count: %w", err)
	}
	if updated == 0 {
		return fmt.Errorf("payment %d not found", payment.ID)
	}
	return nil
}

func (s *Storage) DeletePayment(ctx context.Context, id int) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM PAYMENT WHERE ID = $1`, id)
	if err != nil {
		return fmt.Errorf("delete payment: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get deleted payment count: %w", err)
	}
	if deleted == 0 {
		return fmt.Errorf("payment %d not found", id)
	}
	return nil
}

func paymentsFromRows(rows []paymentRow) []Payment {
	payments := make([]Payment, len(rows))
	for i, row := range rows {
		payments[i] = paymentFromRow(row)
	}
	return payments
}

func paymentFromRow(row paymentRow) Payment {
	return Payment{
		ID:          row.ID,
		ProjectID:   row.ProjectID,
		AmountMinor: row.AmountMinor,
		Currency:    row.Currency,
		PaidAt:      time.Unix(row.PaidAt, 0).UTC(),
		Note:        row.Note,
	}
}

func canonicalPaymentDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func paymentDateInLocation(value time.Time, location *time.Location) time.Time {
	return time.Date(
		value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, location,
	)
}

func (s *Storage) validatePaymentCurrency(
	ctx context.Context,
	projectID int,
	currency string,
) error {
	var projectExists bool
	if err := s.db.GetContext(ctx, &projectExists, `
		SELECT EXISTS (SELECT 1 FROM PROJECT WHERE ID = $1)
	`, projectID); err != nil {
		return fmt.Errorf("check payment project: %w", err)
	}
	if !projectExists {
		return fmt.Errorf("project %d not found", projectID)
	}

	var allowed bool
	if err := s.db.GetContext(ctx, &allowed, `
		SELECT EXISTS (
			SELECT 1
			FROM RATE
			WHERE CURRENCY = $2
			  AND (
				ID = (SELECT RATE_ID FROM PROJECT WHERE ID = $1)
				OR ID IN (
					SELECT RATE_ID
					FROM ENTRY
					WHERE PROJECT_ID = $1 AND RATE_ID IS NOT NULL
				)
			  )
		)
	`, projectID, currency); err != nil {
		return fmt.Errorf("check payment currency: %w", err)
	}
	if !allowed {
		return fmt.Errorf(
			"currency %s is not associated with project %d",
			currency,
			projectID,
		)
	}
	return nil
}
