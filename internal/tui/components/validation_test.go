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

func TestCurrencyAmountInput(t *testing.T) {
	for _, tc := range []struct {
		currency, input string
		want            int
	}{
		{"USD", "125.50", 12550}, {" usd ", " 125.5 ", 12550},
		{"EUR", "0.01", 1}, {"IQD", "125.501", 125501},
		{"JPY", "125", 125}, {"KRW", "0", 0},
		{"USD", "125", 12500}, {"XYZ", "125", 125},
	} {
		t.Run(tc.currency+tc.input, func(t *testing.T) {
			got, err := ParseAmount(tc.input, tc.currency)
			if err != nil || got != tc.want {
				t.Fatalf("amount = %d, error = %v, want %d", got, err, tc.want)
			}
			got, err = ParseAmount(AmountInput(tc.want, tc.currency), tc.currency)
			if err != nil || got != tc.want {
				t.Fatalf("edit round trip = %d, %v", got, err)
			}
		})
	}
	for _, tc := range []struct{ currency, input string }{
		{"USD", "-1"}, {"USD", "1.001"}, {"USD", "1e2"},
		{"USD", "NaN"}, {"USD", "1,000.00"}, {"USD", "."},
		{"JPY", "1.5"}, {"XYZ", "1.5"}, {"US1", "1"},
		{"USD", "999999999999999999999999999.99"},
	} {
		if _, err := ParseAmount(tc.input, tc.currency); err == nil {
			t.Errorf("accepted %q in %s", tc.input, tc.currency)
		}
	}
}
