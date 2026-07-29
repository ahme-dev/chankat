package screens

import (
	"strings"
	"testing"
	"time"

	"chankat/internal/storage"
)

func TestProjectItems(t *testing.T) {
	projectID := 1
	rateID := 1
	endedAt := time.Unix(1_700_000_000, 0)
	items := projectItems(
		[]storage.Project{{ID: projectID, Name: "Client", RateID: rateID}},
		[]storage.Rate{{
			ID: rateID, Name: "Standard", AmountMinor: 5000, Currency: "USD",
		}},
		[]storage.Entry{{
			ProjectID: &projectID,
			RateID:    &rateID,
			StartedAt: endedAt.Add(-time.Hour),
			EndedAt:   &endedAt,
		}},
		[]storage.Payment{{
			ProjectID: projectID, AmountMinor: 2000, Currency: "USD",
		}},
	)

	description := items[0].Description()
	for _, expected := range []string{
		"$30.00 outstanding",
		"1h 00m tracked",
		"Standard Rate · $50.00/h",
	} {
		if !strings.Contains(description, expected) {
			t.Fatalf("description %q does not contain %q", description, expected)
		}
	}
}
