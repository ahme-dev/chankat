package screens

import (
	"testing"

	"chankat/internal/storage"
)

func TestRateItems(t *testing.T) {
	rate := storage.Rate{
		ID: 1, Name: "Standard", AmountMinor: 5000, Currency: "USD",
	}
	items := rateItems(
		[]storage.Rate{rate},
		[]storage.Project{
			{ID: 1, RateID: rate.ID},
			{ID: 2, RateID: rate.ID},
		},
	)

	if got := items[0].Title(); got != "Standard · $50.00/h" {
		t.Fatalf("got title %q", got)
	}
	if got := items[0].Description(); got != "Used by 2 projects" {
		t.Fatalf("got description %q", got)
	}
}
