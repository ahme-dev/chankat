package storage_test

import (
	"strings"
	"testing"
	"time"

	"chankat/internal/storage"
)

func TestGetRates(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		stor := fixtureStorage(t)

		rates, err := stor.GetRates(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(rates) != 0 {
			t.Fatalf("got %d rates, want 0", len(rates))
		}
	})

	t.Run("ordered by ID", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()

		want := []storage.Rate{
			{Name: "standard", AmountMinor: 7500, Currency: "USD"},
			{Name: "discount", AmountMinor: 5000, Currency: "USD"},
		}
		for _, rate := range want {
			if err := stor.CreateRate(ctx, rate); err != nil {
				t.Fatalf("create rate: %v", err)
			}
		}

		got, err := stor.GetRates(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("got %d rates, want %d", len(got), len(want))
		}

		for i := range want {
			if got[i].ID == 0 {
				t.Errorf("rate %d has no ID", i)
			}
			assertRate(t, got[i], want[i])
		}
	})
}

func TestCreateRate(t *testing.T) {
	t.Run("persists rate", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()

		want := storage.Rate{
			Name:        "standard",
			AmountMinor: 7500,
			Currency:    "USD",
		}
		if err := stor.CreateRate(ctx, want); err != nil {
			t.Fatal(err)
		}

		rates, err := stor.GetRates(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(rates) != 1 {
			t.Fatalf("got %d rates, want 1", len(rates))
		}
		if rates[0].ID == 0 {
			t.Error("created rate has no ID")
		}
		assertRate(t, rates[0], want)
	})

	t.Run("rejects invalid amounts and currencies", func(t *testing.T) {
		stor := fixtureStorage(t)
		tests := []storage.Rate{
			{Name: "negative", AmountMinor: -1, Currency: "USD"},
			{Name: "currency", AmountMinor: 1, Currency: "US1"},
		}
		for _, rate := range tests {
			if err := stor.CreateRate(t.Context(), rate); err == nil {
				t.Fatalf("CreateRate(%#v) succeeded", rate)
			}
		}
	})

	t.Run("normalizes currency", func(t *testing.T) {
		stor := fixtureStorage(t)
		if err := stor.CreateRate(t.Context(), storage.Rate{
			Name: "standard", AmountMinor: 100, Currency: " usd ",
		}); err != nil {
			t.Fatal(err)
		}
		rates, err := stor.GetRates(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if got := rates[0].Currency; got != "USD" {
			t.Fatalf("got currency %q, want USD", got)
		}
	})
}

func TestGetRate(t *testing.T) {
	t.Run("existing rate", func(t *testing.T) {
		stor := fixtureStorage(t)
		want := fixtureRate(t, stor)

		got, err := stor.GetRate(t.Context(), want.ID)
		if err != nil {
			t.Fatal(err)
		}
		assertRate(t, got, want)
	})

	t.Run("missing rate", func(t *testing.T) {
		stor := fixtureStorage(t)

		_, err := stor.GetRate(t.Context(), 999)
		if err == nil || !strings.Contains(err.Error(), "rate 999 not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestUpdateRate(t *testing.T) {
	t.Run("updates existing rate", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()

		if err := stor.CreateRate(ctx, storage.Rate{
			Name:        "standard",
			AmountMinor: 7500,
			Currency:    "USD",
		}); err != nil {
			t.Fatal(err)
		}

		rates, err := stor.GetRates(ctx)
		if err != nil {
			t.Fatal(err)
		}

		want := storage.Rate{
			ID:          rates[0].ID,
			Name:        "revised",
			AmountMinor: 8000,
			Currency:    "EUR",
		}
		if err := stor.UpdateRate(ctx, want); err != nil {
			t.Fatal(err)
		}

		rates, err = stor.GetRates(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(rates) != 1 {
			t.Fatalf("got %d rates, want 1", len(rates))
		}
		assertRate(t, rates[0], want)
	})

	t.Run("missing rate", func(t *testing.T) {
		stor := fixtureStorage(t)

		err := stor.UpdateRate(t.Context(), storage.Rate{
			ID:          999,
			Name:        "missing",
			AmountMinor: 7500,
			Currency:    "USD",
		})
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "rate 999 not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("referenced rate", func(t *testing.T) {
		stor := fixtureStorage(t)
		ctx := t.Context()
		rate := fixtureRate(t, stor)
		startedAt := time.Now()

		if err := stor.CreateEntry(ctx, storage.Entry{
			RateID:    &rate.ID,
			StartedAt: startedAt,
		}); err != nil {
			t.Fatal(err)
		}

		rate.AmountMinor = 8000
		err := stor.UpdateRate(ctx, rate)
		if err == nil || !strings.Contains(err.Error(), "referenced") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDeleteRate(t *testing.T) {
	t.Run("existing rate", func(t *testing.T) {
		stor := fixtureStorage(t)
		rate := fixtureRate(t, stor)

		if err := stor.DeleteRate(t.Context(), rate.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := stor.GetRate(t.Context(), rate.ID); err == nil {
			t.Fatal("deleted rate still exists")
		}
	})

	t.Run("missing rate", func(t *testing.T) {
		stor := fixtureStorage(t)

		err := stor.DeleteRate(t.Context(), 999)
		if err == nil || !strings.Contains(err.Error(), "rate 999 not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func assertRate(t *testing.T, got, want storage.Rate) {
	t.Helper()

	if got.Name != want.Name ||
		got.AmountMinor != want.AmountMinor ||
		got.Currency != want.Currency {
		t.Errorf("got rate %#v, want %#v", got, want)
	}
}
