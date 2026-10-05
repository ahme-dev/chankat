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
		Note:        "June",
	}
	item := paymentItems([]storage.Payment{payment}, []storage.Project{project})[0]

	if got := item.Title(); got != "Client · $1,500.50" {
		t.Fatalf("got title %q", got)
	}
	for _, value := range []string{"2026-07-20", "June"} {
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

func TestPaymentRowsNewestPaidFirst(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.AddDate(0, 1, 0)
	payments := []storage.Payment{{ID: 9, PaidAt: older}, {ID: 1, PaidAt: newer}, {ID: 2, PaidAt: newer}}
	items := paymentItems(payments, nil)
	for i, want := range []int{2, 1, 9} {
		if items[i].payment.ID != want {
			t.Fatalf("row %d = %d, want %d", i, items[i].payment.ID, want)
		}
	}
	if payments[0].ID != 9 {
		t.Fatal("row sorting changed storage data")
	}
}
