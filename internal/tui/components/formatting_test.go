package components

import (
	"testing"
	"time"
)

func TestFormatMoney(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		if got := FormatMoney(5000, "USD"); got != "50.00 USD" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("negative", func(t *testing.T) {
		if got := FormatMoney(-125, "EUR"); got != "-1.25 EUR" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestFormatDate(t *testing.T) {
	value := time.Date(2026, 7, 20, 12, 30, 0, 0, time.UTC)
	if got := FormatDate(value); got != "2026-07-20" {
		t.Fatalf("got %q", got)
	}
}
