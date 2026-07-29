package screens

import (
	"testing"

	"chansat/internal/storage"
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

	if got := items[0].Description(); got != "$50.00/h · USD · 2 projects" {
		t.Fatalf("got description %q", got)
	}
}
