package storage

import (
	"fmt"
	"strings"
	"time"
)

func NormalizeCurrency(currency string) (string, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if len(currency) != 3 {
		return "", fmt.Errorf("currency must be a three-letter code")
	}
	for _, character := range currency {
		if character < 'A' || character > 'Z' {
			return "", fmt.Errorf("currency must be a three-letter code")
		}
	}
	return currency, nil
}

func NormalizeName(name, field string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	return name, nil
}

func ValidateAmountMinor(amountMinor int) error {
	if amountMinor < 0 {
		return fmt.Errorf("amount must be non-negative")
	}
	return nil
}

func ValidateEntryTimes(startedAt time.Time, endedAt *time.Time) error {
	if startedAt.IsZero() {
		return fmt.Errorf("start time is required")
	}
	if endedAt != nil && !endedAt.After(startedAt) {
		return fmt.Errorf("end time must be after start time")
	}
	return nil
}

func validatePayment(payment Payment) error {
	if err := ValidateAmountMinor(payment.AmountMinor); err != nil {
		return err
	}
	if payment.PaidAt.IsZero() {
		return fmt.Errorf("paid-at date is required")
	}
	if payment.PaidForDate.IsZero() {
		return fmt.Errorf("paid-for date is required")
	}
	return nil
}

func validateEndTime(endedAt time.Time) error {
	if endedAt.IsZero() {
		return fmt.Errorf("end time is required")
	}
	return nil
}
