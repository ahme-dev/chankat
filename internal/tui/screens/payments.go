package screens

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"chankat/internal/storage"
	"chankat/internal/tui/components"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type Payments struct {
	components.Page[paymentItem]
}

type paymentItem struct {
	payment storage.Payment
	project storage.Project
}

func (p paymentItem) Title() string {
	return p.project.Name + " · " + components.FormatMoney(
		int64(p.payment.AmountMinor),
		p.payment.Currency,
	)
}

func (p paymentItem) Description() string {
	description := "paid " + components.FormatDate(p.payment.PaidAt)
	if p.payment.Note != "" {
		description += " · " + p.payment.Note
	}
	return description
}

func (p paymentItem) FilterValue() string {
	return p.project.Name + " " + p.payment.Note
}

func paymentItems(
	payments []storage.Payment,
	projects []storage.Project,
) []paymentItem {
	projectsByID := make(map[int]storage.Project, len(projects))
	for _, project := range projects {
		projectsByID[project.ID] = project
	}

	items := make([]paymentItem, len(payments))
	for i, payment := range payments {
		items[i] = paymentItem{
			payment: payment,
			project: projectsByID[payment.ProjectID],
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].payment.PaidAt.Equal(items[j].payment.PaidAt) {
			return items[i].payment.PaidAt.After(items[j].payment.PaidAt)
		}
		return items[i].payment.ID > items[j].payment.ID
	})
	return items
}

func NewPayments(ctx context.Context, stor *storage.Storage) Payments {
	config := components.Config[paymentItem]{
		Name: "payments",
		Load: func(ctx context.Context) ([]paymentItem, any, error) {
			payments, err := stor.GetPayments(ctx)
			if err != nil {
				return nil, nil, err
			}
			projects, err := stor.GetProjects(ctx)
			if err != nil {
				return nil, nil, err
			}
			return paymentItems(payments, projects), projects, nil
		},
		Create: func(meta any) (*components.Form[paymentItem], error) {
			return paymentForm(ctx, stor, nil, meta.([]storage.Project))
		},
		Update: func(item paymentItem, meta any) (*components.Form[paymentItem], error) {
			return paymentForm(ctx, stor, &item.payment, meta.([]storage.Project))
		},
		Delete: func(item paymentItem) *components.Form[paymentItem] {
			return deletePaymentForm(ctx, stor, item.payment)
		},
	}
	return Payments{Page: components.NewPage(ctx, config)}
}

func (m Payments) Update(msg tea.Msg) (Payments, tea.Cmd) {
	page, cmd := m.Page.Update(msg)
	m.Page = page
	return m, cmd
}

func paymentForm(
	ctx context.Context,
	stor *storage.Storage,
	payment *storage.Payment,
	projects []storage.Project,
) (*components.Form[paymentItem], error) {
	if len(projects) == 0 {
		return nil, errors.New("no projects available; create a project first")
	}

	today := components.FormatDate(time.Now())
	values := storage.Payment{ProjectID: projects[0].ID, Currency: "USD"}
	amount := ""
	paidAt := today
	action := "new"
	if payment != nil {
		values = *payment
		amount = components.AmountInput(payment.AmountMinor, payment.Currency)
		paidAt = components.FormatDate(payment.PaidAt)
		action = "edit"
	}

	options := make([]huh.Option[int], len(projects))
	for i, project := range projects {
		options[i] = huh.NewOption(project.Name, project.ID)
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[int]().
			Title("Project").
			Options(options...).
			Value(&values.ProjectID),
		huh.NewInput().
			Title("Currency").
			Placeholder("USD").
			CharLimit(3).
			Value(&values.Currency).
			Validate(components.CurrencyCode),
		huh.NewInput().
			Title("Amount").
			Description("Enter a currency amount, e.g. 125.50 USD. Unsupported currencies use minor units.").
			Value(&amount).
			Validate(func(value string) error { _, err := components.ParseAmount(value, values.Currency); return err }),
		huh.NewInput().
			Title("Paid at (YYYY-MM-DD)").
			Value(&paidAt).
			Validate(components.Date),
		huh.NewInput().
			Title("Note").
			Value(&values.Note),
	)).WithShowHelp(true)

	return components.NewForm[paymentItem](
		ctx,
		"payments / "+action,
		form,
		func(ctx context.Context) error {
			parsedAmount, err := components.ParseAmount(amount, values.Currency)
			if err != nil {
				return err
			}
			values.AmountMinor = parsedAmount
			values.Note = strings.TrimSpace(values.Note)
			parsedPaidAt, err := components.ParseDate(paidAt)
			if err != nil {
				return err
			}
			values.PaidAt = parsedPaidAt
			if payment == nil {
				return stor.CreatePayment(ctx, values)
			}
			return stor.UpdatePayment(ctx, values)
		},
	), nil
}

func deletePaymentForm(
	ctx context.Context,
	stor *storage.Storage,
	payment storage.Payment,
) *components.Form[paymentItem] {
	return components.NewDeleteForm[paymentItem](
		ctx,
		components.FormatMoney(int64(payment.AmountMinor), payment.Currency)+" payment",
		func(ctx context.Context) error {
			return stor.DeletePayment(ctx, payment.ID)
		},
	)
}
