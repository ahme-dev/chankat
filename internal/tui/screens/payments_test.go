package screens

import (
	"strings"
	"testing"
	"time"

	"chankat/internal/storage"
	"chankat/internal/tui/components"
)

func TestPaymentItem(t *testing.T) {
	project := storage.Project{ID: 1, Name: "Client"}
	payment := storage.Payment{
		ProjectID:   project.ID,
		AmountMinor: 150_050,
		Currency:    "USD",
		PaidAt:      time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
		PaidForDate: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		Note:        "June",
	}
	item := paymentItems([]storage.Payment{payment}, []storage.Project{project})[0]

	if got := item.Title(); got != "$1,500.50 · Client" {
		t.Fatalf("got title %q", got)
	}
	for _, value := range []string{"2026-07-20", "2026-06-30", "June"} {
		if !strings.Contains(item.Description(), value) {
			t.Fatalf("description %q does not contain %q", item.Description(), value)
		}
	}
}

func TestPaymentDate(t *testing.T) {
	if err := components.Date("2026-07-20"); err != nil {
		t.Fatalf("valid date rejected: %v", err)
	}
	if err := components.Date("20/07/2026"); err == nil {
		t.Fatal("invalid date accepted")
	}
}
