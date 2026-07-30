package components

import (
	"fmt"
	"strings"
	"time"
)

const (
	DateLayout     = "2006-01-02"
	DateTimeLayout = "2006-01-02 15:04"
)

type currencyFormat struct {
	symbol     string
	minorUnits int
}

var currencies = map[string]currencyFormat{
	"AUD": {symbol: "A$", minorUnits: 2},
	"CAD": {symbol: "CA$", minorUnits: 2},
	"CNY": {symbol: "CN¥", minorUnits: 2},
	"EUR": {symbol: "€", minorUnits: 2},
	"GBP": {symbol: "£", minorUnits: 2},
	"INR": {symbol: "₹", minorUnits: 2},
	"IQD": {minorUnits: 3},
	"JPY": {symbol: "¥", minorUnits: 0},
	"KRW": {symbol: "₩", minorUnits: 0},
	"NZD": {symbol: "NZ$", minorUnits: 2},
	"USD": {symbol: "$", minorUnits: 2},
}

func FormatMoney(amountMinor int64, currency string) string {
	sign := ""
	if amountMinor < 0 {
		sign = "-"
		amountMinor = -amountMinor
	}
	code := strings.ToUpper(strings.TrimSpace(currency))
	format, known := currencies[code]
	if !known {
		amount := groupMoneyDigits(fmt.Sprintf("%s%d", sign, amountMinor))
		return amount + " " + code + " minor"
	}
	scale := int64(1)
	for range format.minorUnits {
		scale *= 10
	}
	amount := fmt.Sprintf("%s%d", sign, amountMinor/scale)
	if format.minorUnits > 0 {
		amount += fmt.Sprintf(".%0*d", format.minorUnits, amountMinor%scale)
	}
	amount = groupMoneyDigits(amount)
	if format.symbol != "" {
		if sign != "" {
			return sign + format.symbol + strings.TrimPrefix(amount, sign)
		}
		return format.symbol + amount
	}
	return amount + " " + code
}

func groupMoneyDigits(amount string) string {
	offset := 0
	if strings.HasPrefix(amount, "-") {
		offset = 1
	}
	decimal := strings.IndexByte(amount, '.')
	if decimal == -1 {
		decimal = len(amount)
	}
	for position := decimal - 3; position > offset; position -= 3 {
		amount = amount[:position] + "," + amount[position:]
	}
	return amount
}

func FormatDate(value time.Time) string {
	return value.Format(DateLayout)
}

func FormatDateTime(value time.Time) string {
	return value.Format(DateTimeLayout)
}

func FormatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	hours := int(duration / time.Hour)
	minutes := int(duration%time.Hour) / int(time.Minute)
	return fmt.Sprintf("%dh %02dm", hours, minutes)
}
