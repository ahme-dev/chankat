package components

import "github.com/charmbracelet/lipgloss"

const AccentColor lipgloss.Color = "208"

var chartColors = [...]lipgloss.Color{
	AccentColor,
	"39",
	"42",
	"141",
	"220",
	"81",
}

func ChartColor(index int) lipgloss.Color {
	return chartColors[index%len(chartColors)]
}
