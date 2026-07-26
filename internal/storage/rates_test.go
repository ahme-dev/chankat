package storage_test

import (
	"chansat/internal/storage"
	"testing"
)

func TestCreateRate(t *testing.T) {
	stor := fixtureStorage(t)
	ctx := t.Context()

	err := stor.CreateRate(ctx, storage.Rate{
		Name:        "standard",
		AmountMinor: 7500,
		Currency:    "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListRates(t *testing.T) {
	stor := fixtureStorage(t)
	ctx := t.Context()

	expected := []storage.Rate{}

	got, err := stor.GetRates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(expected) {
		t.Fatalf("expected %d rate(s), got %d", len(expected), len(got))
	}

	err = stor.CreateRate(ctx, storage.Rate{
		Name:        "standard",
		AmountMinor: 7500,
		Currency:    "USD",
	})
	if err != nil {
		t.Fatal(err)
	}

	expected = []storage.Rate{{
		Name:        "standard",
		AmountMinor: 7500,
		Currency:    "USD",
	}}

	for i, gotRate := range got {
		if gotRate.Name != expected[i].Name || gotRate.AmountMinor != expected[i].AmountMinor || gotRate.Currency != expected[i].Currency {
			t.Fatalf("expected %v, got %v", expected[i], gotRate)
		}
	}
}
