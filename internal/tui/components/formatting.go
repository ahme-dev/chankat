package components

import (
	"fmt"
	"time"
)

const DateLayout = "2006-01-02"

func FormatMoney(amountMinor int64, currency string) string {
	sign := ""
	if amountMinor < 0 {
		sign = "-"
		amountMinor = -amountMinor
	}
	return fmt.Sprintf(
		"%s%d.%02d %s",
		sign,
		amountMinor/100,
		amountMinor%100,
		currency,
	)
}

func FormatDate(value time.Time) string {
	return value.Format(DateLayout)
}

func FormatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	hours := int(duration / time.Hour)
	minutes := int(duration%time.Hour) / int(time.Minute)
	return fmt.Sprintf("%dh %02dm", hours, minutes)
}
