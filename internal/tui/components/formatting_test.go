package components

import (
	"testing"
	"time"
)

func TestFormatMoney(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		if got := FormatMoney(5000, "USD"); got != "$50.00" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("negative", func(t *testing.T) {
		if got := FormatMoney(-125, "EUR"); got != "-€1.25" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("distinguishes dollar currencies", func(t *testing.T) {
		if got := FormatMoney(5000, "CAD"); got != "CA$50.00" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("retains unknown code", func(t *testing.T) {
		if got := FormatMoney(5000, "XYZ"); got != "5,000 XYZ minor" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("groups thousands", func(t *testing.T) {
		if got := FormatMoney(123_456_789, "USD"); got != "$1,234,567.89" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("three minor units", func(t *testing.T) {
		if got := FormatMoney(12_345, "IQD"); got != "12.345 IQD" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("no minor units", func(t *testing.T) {
		if got := FormatMoney(123_456, "JPY"); got != "¥123,456" {
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

func TestDateTime(t *testing.T) {
	value := time.Date(2026, 7, 20, 12, 30, 0, 0, time.Local)
	if got := FormatDateTime(value); got != "2026-07-20 12:30" {
		t.Fatalf("got %q", got)
	}
	parsed, err := ParseDateTime(" 2026-07-20 12:30 ")
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.Equal(value) {
		t.Fatalf("got %v, want %v", parsed, value)
	}
	if err := DateTime("20/07/2026 12:30"); err == nil {
		t.Fatal("invalid datetime accepted")
	}
	if err := OptionalDateTime(""); err != nil {
		t.Fatalf("blank optional datetime rejected: %v", err)
	}
}
