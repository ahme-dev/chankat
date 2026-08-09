package components

import (
	"fmt"
	"time"

	"chankat/internal/storage"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

const customPeriod = "custom"

type PeriodMenu struct {
	form *huh.Form
	kind string
	from string
	to   string
}

func NewPeriodMenu(
	period storage.Period,
	now time.Time,
	width int,
	fields ...huh.Field,
) *PeriodMenu {
	start, end := now, now
	if !period.Start.IsZero() {
		start = period.Start
	}
	if !period.End.IsZero() {
		end = period.End.AddDate(0, 0, -1)
	}
	kind := string(period.Kind)
	if kind == "" {
		kind = customPeriod
	}
	m := &PeriodMenu{
		kind: kind,
		from: FormatDate(start),
		to:   FormatDate(end),
	}
	fields = append(fields,
		huh.NewSelect[string]().Title("Period").Options(
			huh.NewOption("All time", string(storage.All)),
			huh.NewOption("Day", string(storage.Day)),
			huh.NewOption("Week", string(storage.Week)),
			huh.NewOption("Month", string(storage.Month)),
			huh.NewOption("Custom", customPeriod),
		).Value(&m.kind),
	)
	filters := huh.NewGroup(fields...)
	dates := huh.NewGroup(
		huh.NewInput().Title("From (YYYY-MM-DD)").Value(&m.from).
			Validate(Date),
		huh.NewInput().Title("To (YYYY-MM-DD, inclusive)").Value(&m.to).
			Validate(func(value string) error {
				if err := Date(value); err != nil {
					return err
				}
				start, err := ParseDate(m.from)
				if err != nil {
					return err
				}
				end, err := ParseDate(value)
				if err != nil {
					return err
				}
				if end.Before(start) {
					return fmt.Errorf("end date must not precede start date")
				}
				return nil
			}),
	).WithHideFunc(func() bool { return m.kind != customPeriod })
	m.form = huh.NewForm(filters, dates).WithShowHelp(true).WithWidth(width)
	return m
}

func (m *PeriodMenu) Init() tea.Cmd { return m.form.Init() }

func (m *PeriodMenu) Update(msg tea.Msg) tea.Cmd {
	updated, cmd := m.form.Update(msg)
	m.form = updated.(*huh.Form)
	return cmd
}

func (m *PeriodMenu) View() string { return m.form.View() }

func (m *PeriodMenu) Completed() bool { return m.form.State == huh.StateCompleted }

func (m *PeriodMenu) Aborted() bool { return m.form.State == huh.StateAborted }

func (m *PeriodMenu) Period(now time.Time) (storage.Period, error) {
	if m.kind != customPeriod {
		return storage.CurrentPeriod(storage.PeriodKind(m.kind), now)
	}
	start, err := ParseDate(m.from)
	if err != nil {
		return storage.Period{}, err
	}
	end, err := ParseDate(m.to)
	if err != nil {
		return storage.Period{}, err
	}
	return storage.CustomPeriod(start, end)
}
