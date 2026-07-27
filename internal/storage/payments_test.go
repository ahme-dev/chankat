package storage_test

import (
	"strings"
	"testing"
	"time"

	"chansat/internal/storage"
)

func TestCreatePayment(t *testing.T) {
	t.Run("persists receipt and accounting dates", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		paidAt := time.Unix(1_706_745_600, 0)
		paidFor := time.Unix(1_704_067_200, 0)

		if err := stor.CreatePayment(t.Context(), storage.Payment{
			ProjectID: project.ID, AmountMinor: 150_000, Currency: "USD",
			PaidAt: paidAt, PaidForDate: paidFor, Note: "January",
		}); err != nil {
			t.Fatal(err)
		}
		payment, err := stor.GetPayment(t.Context(), 1)
		if err != nil {
			t.Fatal(err)
		}
		if !payment.PaidAt.Equal(paidAt) || !payment.PaidForDate.Equal(paidFor) {
			t.Fatalf("unexpected payment dates: %#v", payment)
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
	t.Run("updates paid-for date", func(t *testing.T) {
		stor := fixtureStorage(t)
		project := fixtureProject(t, stor)
		paidAt := time.Unix(1_706_745_600, 0)
		paidFor := time.Unix(1_704_067_200, 0)
		payment := storage.Payment{
			ProjectID: project.ID, AmountMinor: 150_000, Currency: "USD",
			PaidAt: paidAt, PaidForDate: paidFor,
		}
		if err := stor.CreatePayment(t.Context(), payment); err != nil {
			t.Fatal(err)
		}
		payment.ID = 1
		payment.PaidForDate = paidFor.AddDate(0, 1, 0)
		if err := stor.UpdatePayment(t.Context(), payment); err != nil {
			t.Fatal(err)
		}
		got, err := stor.GetPayment(t.Context(), 1)
		if err != nil {
			t.Fatal(err)
		}
		if !got.PaidForDate.Equal(payment.PaidForDate) {
			t.Fatalf("got paid-for date %v, want %v", got.PaidForDate, payment.PaidForDate)
		}
	})
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
