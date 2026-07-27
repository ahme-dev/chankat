package components

import (
	"errors"
	"strconv"
	"strings"
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
