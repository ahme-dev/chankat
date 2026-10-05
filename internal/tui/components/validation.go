package components

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chankat/internal/storage"
)

func Required(name string) func(string) error {
	return func(value string) error {
		if strings.TrimSpace(value) == "" {
			return errors.New(name + " is required")
		}
		return nil
	}
}

func NonNegativeAmount(value string) error {
	_, err := ParseAmountMinor(value)
	return err
}

func ParseAmountMinor(value string) (int, error) {
	amount, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, errors.New("amount must be a non-negative integer")
	}
	if err := storage.ValidateAmountMinor(amount); err != nil {
		return 0, errors.New("amount must be a non-negative integer")
	}
	return amount, nil
}

// AmountInput formats an editable amount without symbols or grouping.
// Unsupported currencies retain the application's existing minor-unit convention.
func AmountInput(amount int, currency string) string {
	precision := currencies[strings.ToUpper(strings.TrimSpace(currency))].minorUnits
	scale := 1
	for range precision {
		scale *= 10
	}
	if precision == 0 {
		return strconv.Itoa(amount)
	}
	return fmt.Sprintf("%d.%0*d", amount/scale, precision, amount%scale)
}

func ParseAmount(value, currency string) (int, error) {
	if err := CurrencyCode(currency); err != nil {
		return 0, err
	}
	precision := currencies[strings.ToUpper(strings.TrimSpace(currency))].minorUnits
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("enter a non-negative amount, such as 125.50")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > precision || (len(parts) == 2 && (fraction == "" || precision == 0)) {
		return 0, fmt.Errorf("%s amounts allow %d decimal places", strings.ToUpper(strings.TrimSpace(currency)), precision)
	}
	for _, part := range parts {
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return 0, errors.New("amount must contain only digits and a decimal point")
			}
		}
	}
	return ParseAmountMinor(parts[0] + fraction + strings.Repeat("0", precision-len(fraction)))
}

func CurrencyCode(value string) error {
	_, err := storage.NormalizeCurrency(value)
	return err
}

func Date(value string) error {
	if _, err := ParseDate(value); err != nil {
		return errors.New("date must use YYYY-MM-DD")
	}
	return nil
}

func ParseDate(value string) (time.Time, error) {
	return time.ParseInLocation(DateLayout, strings.TrimSpace(value), time.Local)
}

func DateTime(value string) error {
	if _, err := ParseDateTime(value); err != nil {
		return errors.New("time must use YYYY-MM-DD HH:MM")
	}
	return nil
}

func OptionalDateTime(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return DateTime(value)
}

func ParseDateTime(value string) (time.Time, error) {
	return time.ParseInLocation(DateTimeLayout, strings.TrimSpace(value), time.Local)
}

func EntryEndTime(startedAt *string, optional bool) func(string) error {
	return func(value string) error {
		if optional && strings.TrimSpace(value) == "" {
			return nil
		}
		if err := DateTime(value); err != nil {
			return err
		}
		started, err := ParseDateTime(*startedAt)
		if err != nil {
			return errors.New("enter a valid start time first")
		}
		ended, err := ParseDateTime(value)
		if err != nil {
			return err
		}
		return storage.ValidateEntryTimes(started, &ended)
	}
}
