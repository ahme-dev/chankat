package storage_test

import (
	"strings"
	"testing"
	"time"

	"chankat/internal/storage"
)

func TestCreatePayment(t *testing.T) {
	t.Run("persists receipt date and keeps legacy date compatible", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		paidAt := time.Unix(1_706_745_600, 0)

		if err := stor.CreatePayment(t.Context(), storage.Payment{
			ProjectID: project.ID, AmountMinor: 150_000, Currency: "USD",
			PaidAt: paidAt, Note: "January",
		}); err != nil {
			t.Fatal(err)
		}
		payment, err := stor.GetPayment(t.Context(), 1)
		if err != nil {
			t.Fatal(err)
		}
		expectedDate := canonicalTestDate(paidAt)
		if !payment.PaidAt.Equal(expectedDate) {
			t.Fatalf("unexpected payment date: %#v", payment)
		}
		var legacyDate int64
		if err := stor.QueryRow(
			`SELECT PAID_FOR_DATE FROM PAYMENT WHERE ID = 1`,
		).Scan(&legacyDate); err != nil {
			t.Fatal(err)
		}
		if legacyDate != expectedDate.Unix() {
			t.Fatalf("legacy date = %d, want %d", legacyDate, expectedDate.Unix())
		}
	})

	t.Run("rejects invalid financial data", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		date := time.Unix(1_706_745_600, 0)
		tests := []storage.Payment{
			{
				ProjectID: project.ID, AmountMinor: -1, Currency: "USD",
				PaidAt: date,
			},
			{
				ProjectID: project.ID, AmountMinor: 1, Currency: "US1",
				PaidAt: date,
			},
			{
				ProjectID: project.ID, AmountMinor: 1, Currency: "USD",
			},
		}
		for _, payment := range tests {
			if err := stor.CreatePayment(t.Context(), payment); err == nil {
				t.Fatalf("CreatePayment(%#v) succeeded", payment)
			}
		}
	})

	t.Run("rejects a currency unrelated to the project", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		err := stor.CreatePayment(t.Context(), storage.Payment{
			ProjectID: project.ID, AmountMinor: 1, Currency: "EUR",
			PaidAt: time.Unix(1_706_745_600, 0),
		})
		if err == nil || !strings.Contains(err.Error(), "not associated") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("allows a currency captured by historical work", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		if err := stor.CreateRate(t.Context(), storage.Rate{
			Name: "old euro", AmountMinor: 10_000, Currency: "EUR",
		}); err != nil {
			t.Fatal(err)
		}
		rates, err := stor.GetRates(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		rateID := rates[len(rates)-1].ID
		projectID := project.ID
		endedAt := time.Unix(1_706_745_600, 0)
		if err := stor.CreateEntry(t.Context(), storage.Entry{
			ProjectID: &projectID, RateID: &rateID,
			StartedAt: endedAt.Add(-time.Hour), EndedAt: &endedAt,
		}); err != nil {
			t.Fatal(err)
		}
		if err := stor.CreatePayment(t.Context(), storage.Payment{
			ProjectID: project.ID, AmountMinor: 1, Currency: "EUR",
			PaidAt: endedAt,
		}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGetPayments(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		stor := fixtureStorage(t)
		payments, err := stor.GetPayments(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(payments) != 0 {
			t.Fatalf("got %d payments, want 0", len(payments))
		}
	})
}

func TestGetPayment(t *testing.T) {
	t.Run("missing payment", func(t *testing.T) {
		stor := fixtureStorage(t)
		_, err := stor.GetPayment(t.Context(), 999)
		if err == nil || !strings.Contains(err.Error(), "payment 999 not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestUpdatePayment(t *testing.T) {
	t.Run("updates receipt date", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		paidAt := time.Unix(1_706_745_600, 0)
		payment := storage.Payment{
			ProjectID: project.ID, AmountMinor: 150_000, Currency: "USD",
			PaidAt: paidAt,
		}
		if err := stor.CreatePayment(t.Context(), payment); err != nil {
			t.Fatal(err)
		}
		payment.ID = 1
		payment.PaidAt = paidAt.AddDate(0, 1, 0)
		if err := stor.UpdatePayment(t.Context(), payment); err != nil {
			t.Fatal(err)
		}
		got, err := stor.GetPayment(t.Context(), 1)
		if err != nil {
			t.Fatal(err)
		}
		expectedDate := canonicalTestDate(payment.PaidAt)
		if !got.PaidAt.Equal(expectedDate) {
			t.Fatalf("got paid-at date %v, want %v", got.PaidAt, payment.PaidAt)
		}
		var legacyDate int64
		if err := stor.QueryRow(
			`SELECT PAID_FOR_DATE FROM PAYMENT WHERE ID = 1`,
		).Scan(&legacyDate); err != nil {
			t.Fatal(err)
		}
		if legacyDate != expectedDate.Unix() {
			t.Fatalf("legacy date = %d, want %d",
				legacyDate, expectedDate.Unix())
		}
	})

	t.Run("allows edits when a historical currency is unchanged", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		payment := storage.Payment{
			ProjectID: project.ID, AmountMinor: 5_000, Currency: "USD",
			PaidAt: time.Date(2026, 7, 20, 0, 0, 0, 0, time.Local),
		}
		if err := stor.CreatePayment(t.Context(), payment); err != nil {
			t.Fatal(err)
		}
		if err := stor.CreateRate(t.Context(), storage.Rate{
			Name: "euro", AmountMinor: 10_000, Currency: "EUR",
		}); err != nil {
			t.Fatal(err)
		}
		rates, err := stor.GetRates(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		project.RateID = rates[len(rates)-1].ID
		if err := stor.UpdateProject(t.Context(), project); err != nil {
			t.Fatal(err)
		}
		payment.ID = 1
		payment.Note = "updated"
		if err := stor.UpdatePayment(t.Context(), payment); err != nil {
			t.Fatal(err)
		}
	})
}

func canonicalTestDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func TestDeletePayment(t *testing.T) {
	t.Run("missing payment", func(t *testing.T) {
		stor := fixtureStorage(t)
		err := stor.DeletePayment(t.Context(), 999)
		if err == nil || !strings.Contains(err.Error(), "payment 999 not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
