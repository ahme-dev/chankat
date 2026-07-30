package cli

import (
	"fmt"
	"strings"
	"time"
)

const (
	dateLayout     = "2006-01-02"
	dateTimeLayout = "2006-01-02 15:04"
)

func parseDate(value string) (time.Time, error) {
	parsed, err := time.ParseInLocation(dateLayout, strings.TrimSpace(value), time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("date must use YYYY-MM-DD: %w", err)
	}
	return parsed, nil
}

func parseDateTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	parsed, err := time.ParseInLocation(dateTimeLayout, value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"time must use RFC3339 or YYYY-MM-DD HH:MM: %w",
			err,
		)
	}
	return parsed, nil
}
