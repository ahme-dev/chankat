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
		"Current rate: Standard · $50.00/h",
	} {
		if !strings.Contains(description, expected) {
			t.Fatalf("description %q does not contain %q", description, expected)
		}
	}
	for _, expected := range []string{"$50.00 earned", "$20.00 received"} {
		if strings.Contains(description, expected) || !strings.Contains(items[0].accountingSummary(), expected) {
			t.Fatalf("accounting breakdown should appear only in editor: %q", description)
		}
	}
}

func TestProjectItemsRoundAfterAggregation(t *testing.T) {
	projectID := 1
	rateID := 1
	startedAt := time.Unix(1_700_000_000, 0)
	firstEnd := startedAt.Add(30 * time.Minute)
	secondEnd := firstEnd.Add(30 * time.Minute)
	items := projectItems(
		[]storage.Project{{ID: projectID, Name: "Client", RateID: rateID}},
		[]storage.Rate{{
			ID: rateID, Name: "Standard", AmountMinor: 1, Currency: "USD",
		}},
		[]storage.Entry{
			{
				ProjectID: &projectID, RateID: &rateID,
				StartedAt: startedAt, EndedAt: &firstEnd,
			},
			{
				ProjectID: &projectID, RateID: &rateID,
				StartedAt: firstEnd, EndedAt: &secondEnd,
			},
		},
		nil,
	)

	if got := items[0].balance["USD"]; got != 1 {
		t.Fatalf("got %d minor units, want 1", got)
	}
}

func TestProjectEditorIncludesAccounting(t *testing.T) {
	rate := storage.Rate{ID: 1, Name: "Standard", AmountMinor: 5000, Currency: "USD"}
	item := projectItem{project: storage.Project{ID: 1, Name: "Client", RateID: 1}, rate: rate,
		balance: map[string]int64{"USD": 3000}, earned: map[string]int64{"USD": 5000}, paid: map[string]int64{"USD": 2000}}
	form, err := projectForm(t.Context(), nil, &item.project, []storage.Rate{rate}, item.accountingSummary())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"projects / edit", "All time through now", "$50.00 earned", "$20.00 received"} {
		if !strings.Contains(form.View(), want) {
			t.Fatalf("editor missing %q: %s", want, form.View())
		}
	}
}

func TestProjectRowsAlphabetical(t *testing.T) {
	items := projectItems([]storage.Project{{ID: 1, Name: "Zulu"}, {ID: 2, Name: "alpha"}}, nil, nil, nil)
	if items[0].project.ID != 2 {
		t.Fatalf("first row = %s", items[0].Title())
	}
}
