package components

import (
	"errors"
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
