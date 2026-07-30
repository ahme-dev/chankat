package components

import (
	"errors"
	"strconv"
	"strings"
	"time"
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
	amount, err := strconv.Atoi(value)
	if err != nil || amount < 0 {
		return errors.New("amount must be a non-negative integer")
	}
	return nil
}

func CurrencyCode(value string) error {
	if len(strings.TrimSpace(value)) != 3 {
		return errors.New("currency must be a three-letter code")
	}
	return nil
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
