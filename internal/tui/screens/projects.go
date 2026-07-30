package screens

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"chankat/internal/storage"
	"chankat/internal/tui/components"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type Projects struct {
	components.Page[projectItem]
}

type projectItem struct {
	project storage.Project
	rate    storage.Rate
	balance map[string]int64
	tracked time.Duration
}

func (p projectItem) Title() string {
	return p.project.Name
}

func (p projectItem) Description() string {
	currencies := make([]string, 0, len(p.balance))
	for currency := range p.balance {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	balances := make([]string, len(currencies))
	for i, currency := range currencies {
		balances[i] = components.FormatMoney(p.balance[currency], currency)
	}
	parts := make([]string, 0, 4)
	if len(balances) > 0 {
		parts = append(parts, strings.Join(balances, ", ")+" outstanding")
	}
	parts = append(
		parts,
		components.FormatDuration(p.tracked)+" tracked",
		fmt.Sprintf(
			"%s Rate · %s/h",
			p.rate.Name,
			components.FormatMoney(int64(p.rate.AmountMinor), p.rate.Currency),
		),
	)
	return strings.Join(parts, " · ")
}

func (p projectItem) FilterValue() string {
	return p.project.Name
}

func projectItems(
	projects []storage.Project,
	rates []storage.Rate,
	entries []storage.Entry,
	payments []storage.Payment,
) []projectItem {
	ratesByID := make(map[int]storage.Rate, len(rates))
	for _, rate := range rates {
		ratesByID[rate.ID] = rate
	}
	balances := make(map[int]map[string]int64, len(projects))
	minorSeconds := make(map[int]map[string]int64, len(projects))
	tracked := make(map[int]time.Duration, len(projects))
	for _, project := range projects {
		balances[project.ID] = map[string]int64{
			ratesByID[project.RateID].Currency: 0,
		}
		minorSeconds[project.ID] = make(map[string]int64)
	}
	for _, entry := range entries {
		if entry.ProjectID == nil || entry.RateID == nil || entry.EndedAt == nil {
			continue
		}
		rate, ok := ratesByID[*entry.RateID]
		if !ok {
			continue
		}
		if _, ok := balances[*entry.ProjectID]; !ok {
			continue
		}
		elapsed := entry.EndedAt.Sub(entry.StartedAt)
		if elapsed < 0 {
			elapsed = 0
		}
		seconds := int64(elapsed / time.Second)
		tracked[*entry.ProjectID] += elapsed
		minorSeconds[*entry.ProjectID][rate.Currency] +=
			int64(rate.AmountMinor) * seconds
	}
	for projectID, currencies := range minorSeconds {
		for currency, total := range currencies {
			balances[projectID][currency] += total / 3600
		}
	}
	for _, payment := range payments {
		if balances[payment.ProjectID] == nil {
			continue
		}
		balances[payment.ProjectID][payment.Currency] -= int64(payment.AmountMinor)
	}

	items := make([]projectItem, len(projects))
	for i, project := range projects {
		items[i] = projectItem{
			project: project,
			rate:    ratesByID[project.RateID],
			balance: balances[project.ID],
			tracked: tracked[project.ID],
		}
	}
	return items
}

func NewProjects(ctx context.Context, stor *storage.Storage) Projects {
	config := components.Config[projectItem]{
		Name: "projects",
		Load: func(ctx context.Context) ([]projectItem, any, error) {
			projects, err := stor.GetProjects(ctx)
			if err != nil {
				return nil, nil, err
			}
			rates, err := stor.GetRates(ctx)
			if err != nil {
				return nil, nil, err
			}
			entries, err := stor.GetEntries(ctx)
			if err != nil {
				return nil, nil, err
			}
			payments, err := stor.GetPayments(ctx)
			if err != nil {
				return nil, nil, err
			}
			return projectItems(projects, rates, entries, payments), rates, nil
		},
		Create: func(meta any) (*components.Form[projectItem], error) {
			return projectForm(ctx, stor, nil, meta.([]storage.Rate))
		},
		Update: func(item projectItem, meta any) (*components.Form[projectItem], error) {
			return projectForm(ctx, stor, &item.project, meta.([]storage.Rate))
		},
		Delete: func(item projectItem) *components.Form[projectItem] {
			return deleteProjectForm(ctx, stor, item.project)
		},
	}
	return Projects{Page: components.NewPage(ctx, config)}
}

func (m Projects) Update(msg tea.Msg) (Projects, tea.Cmd) {
	page, cmd := m.Page.Update(msg)
	m.Page = page
	return m, cmd
}

func projectForm(
	ctx context.Context,
	stor *storage.Storage,
	project *storage.Project,
	rates []storage.Rate,
) (*components.Form[projectItem], error) {
	if len(rates) == 0 {
		return nil, errors.New("no rates available; create a rate first")
	}

	values := storage.Project{RateID: rates[0].ID}
	action := "new"
	if project != nil {
		values = *project
		action = "edit"
	}
	options := make([]huh.Option[int], len(rates))
	for i, rate := range rates {
		label := fmt.Sprintf(
			"%s Rate · %s/h",
			rate.Name,
			components.FormatMoney(int64(rate.AmountMinor), rate.Currency),
		)
		options[i] = huh.NewOption(label, rate.ID)
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Name").
			Value(&values.Name).
			Validate(components.Required("name")),
		huh.NewSelect[int]().
			Title("Rate").
			Options(options...).
			Value(&values.RateID),
	)).WithShowHelp(true)

	return components.NewForm[projectItem](
		ctx,
		"projects / "+action,
		form,
		func(ctx context.Context) error {
			values.Name = strings.TrimSpace(values.Name)
			if project == nil {
				return stor.CreateProject(ctx, values)
			}
			return stor.UpdateProject(ctx, values)
		},
	), nil
}

func deleteProjectForm(
	ctx context.Context,
	stor *storage.Storage,
	project storage.Project,
) *components.Form[projectItem] {
	return components.NewDeleteForm[projectItem](
		ctx,
		project.Name,
		func(ctx context.Context) error {
			return stor.DeleteProject(ctx, project.ID)
		},
	)
}
