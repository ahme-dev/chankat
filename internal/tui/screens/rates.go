package screens

import (
	"context"
	"strconv"
	"strings"

	"chankat/internal/storage"
	"chankat/internal/tui/components"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type Rates struct {
	components.Page[rateItem]
}

type rateItem struct {
	storage.Rate
	projectCount int
}

func (r rateItem) Title() string {
	return r.Name + " Rate"
}

func (r rateItem) Description() string {
	return components.FormatMoney(int64(r.AmountMinor), r.Currency) +
		"/h · " + r.Currency + " · " +
		strconv.Itoa(r.projectCount) + " " +
		plural(r.projectCount, "project")
}

func (r rateItem) FilterValue() string {
	return r.Name
}

func rateItems(rates []storage.Rate, projects []storage.Project) []rateItem {
	projectCounts := make(map[int]int, len(rates))
	for _, project := range projects {
		projectCounts[project.RateID]++
	}
	items := make([]rateItem, len(rates))
	for i, rate := range rates {
		items[i] = rateItem{
			Rate: rate, projectCount: projectCounts[rate.ID],
		}
	}
	return items
}

func plural(count int, singular string) string {
	if count == 1 {
		return singular
	}
	return singular + "s"
}

func NewRates(ctx context.Context, stor *storage.Storage) Rates {
	config := components.Config[rateItem]{
		Name: "rates",
		Load: func(ctx context.Context) ([]rateItem, any, error) {
			rates, err := stor.GetRates(ctx)
			if err != nil {
				return nil, nil, err
			}
			projects, err := stor.GetProjects(ctx)
			if err != nil {
				return nil, nil, err
			}
			return rateItems(rates, projects), nil, nil
		},
		Create: func(any) (*components.Form[rateItem], error) {
			return rateForm(ctx, stor, nil)
		},
		Update: func(item rateItem, _ any) (*components.Form[rateItem], error) {
			return rateForm(ctx, stor, &item.Rate)
		},
		Delete: func(item rateItem) *components.Form[rateItem] {
			return deleteRateForm(ctx, stor, item.Rate)
		},
	}
	return Rates{Page: components.NewPage(ctx, config)}
}

func (m Rates) Update(msg tea.Msg) (Rates, tea.Cmd) {
	page, cmd := m.Page.Update(msg)
	m.Page = page
	return m, cmd
}

func rateForm(
	ctx context.Context,
	stor *storage.Storage,
	rate *storage.Rate,
) (*components.Form[rateItem], error) {
	values := storage.Rate{}
	amountMinor := ""
	action := "new"
	if rate != nil {
		values = *rate
		amountMinor = strconv.Itoa(rate.AmountMinor)
		action = "edit"
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Name").
			Value(&values.Name).
			Validate(components.Required("name")),
		huh.NewInput().
			Title("Amount in minor units").
			Value(&amountMinor).
			Validate(components.NonNegativeAmount),
		huh.NewInput().
			Title("Currency").
			Placeholder("USD").
			CharLimit(3).
			Value(&values.Currency).
			Validate(components.CurrencyCode),
	)).WithShowHelp(true)

	return components.NewForm[rateItem](
		ctx,
		"rates / "+action,
		form,
		func(ctx context.Context) error {
			values.Name = strings.TrimSpace(values.Name)
			values.AmountMinor, _ = strconv.Atoi(amountMinor)
			values.Currency = strings.ToUpper(strings.TrimSpace(values.Currency))
			if rate == nil {
				return stor.CreateRate(ctx, values)
			}
			return stor.UpdateRate(ctx, values)
		},
	), nil
}

func deleteRateForm(
	ctx context.Context,
	stor *storage.Storage,
	rate storage.Rate,
) *components.Form[rateItem] {
	return components.NewDeleteForm[rateItem](
		ctx,
		rate.Name,
		func(ctx context.Context) error {
			return stor.DeleteRate(ctx, rate.ID)
		},
	)
}
