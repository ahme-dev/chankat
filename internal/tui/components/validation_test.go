package components

import "testing"

func TestCurrencyCodeUsesStorageRules(t *testing.T) {
	for _, value := range []string{"USD", " usd "} {
		if err := CurrencyCode(value); err != nil {
			t.Errorf("CurrencyCode(%q): %v", value, err)
		}
	}
	for _, value := range []string{"US1", "US", "USDD"} {
		if err := CurrencyCode(value); err == nil {
			t.Errorf("CurrencyCode(%q) succeeded", value)
		}
	}
}

func TestParseAmountMinorUsesStorageRules(t *testing.T) {
	if amount, err := ParseAmountMinor(" 123 "); err != nil || amount != 123 {
		t.Fatalf("got amount %d, error %v", amount, err)
	}
	for _, value := range []string{"-1", "1.5", "invalid"} {
		if _, err := ParseAmountMinor(value); err == nil {
			t.Errorf("ParseAmountMinor(%q) succeeded", value)
		}
	}
}

func TestEntryEndTimeUsesStorageRules(t *testing.T) {
	startedAt := "2026-07-30 10:00"
	validate := EntryEndTime(&startedAt, false)
	if err := validate("2026-07-30 11:00"); err != nil {
		t.Fatalf("valid interval rejected: %v", err)
	}
	if err := validate("2026-07-30 09:00"); err == nil {
		t.Fatal("end before start accepted")
	}
	if err := validate(""); err == nil {
		t.Fatal("required end accepted as blank")
	}
	if err := EntryEndTime(&startedAt, true)(""); err != nil {
		t.Fatalf("optional end rejected as blank: %v", err)
	}
}
