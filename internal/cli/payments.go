package cli

import (
	"fmt"

	"chankat/internal/storage"
)

func (r runner) runPayments(args []string) error {
	if len(args) == 0 {
		return r.listPayments(nil)
	}
	switch args[0] {
	case "list":
		return r.listPayments(args[1:])
	case "get":
		return r.getPayment(args[1:])
	case "create":
		return r.createPayment(args[1:])
	case "update":
		return r.updatePayment(args[1:])
	case "delete":
		return r.deletePayment(args[1:])
	case "help", "-h", "--help":
		r.help([]string{"payments"})
		return nil
	default:
		return fmt.Errorf("unknown payments command %q", args[0])
	}
}

func (r runner) paymentData() ([]storage.Payment, []storage.Project, error) {
	payments, err := r.stor.GetPayments(r.ctx)
	if err != nil {
		return nil, nil, err
	}
	projects, err := r.stor.GetProjects(r.ctx)
	if err != nil {
		return nil, nil, err
	}
	return payments, projects, nil
}

func (r runner) listPayments(args []string) error {
	if err := requireNoArgs(args, "chankat payments list"); err != nil {
		return err
	}
	payments, projects, err := r.paymentData()
	if err != nil {
		return fmt.Errorf("list payments: %w", err)
	}
	output := paymentOutputs(payments, projects)
	if r.json {
		return r.writeJSON(output)
	}
	rows := make([]string, len(output))
	for i, item := range output {
		rows[i] = fmt.Sprintf("%d\t%d\t%s\t%d\t%s\t%s\t%s\t%s", item.ID,
			item.ProjectID, item.ProjectName, item.AmountMinor, item.Currency,
			item.PaidAt, item.PaidForDate, item.Note)
	}
	return r.table(
		"ID\tPROJECT_ID\tPROJECT\tAMOUNT_MINOR\tCURRENCY\tPAID_AT\tPAID_FOR\tNOTE",
		rows,
	)
}

func (r runner) getPayment(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: chankat payments get ID")
	}
	id, err := parseID(args[0], "payment")
	if err != nil {
		return err
	}
	payment, err := r.stor.GetPayment(r.ctx, id)
	if err != nil {
		return err
	}
	projects, err := r.stor.GetProjects(r.ctx)
	if err != nil {
		return err
	}
	output := paymentOutputs([]storage.Payment{payment}, projects)[0]
	if r.json {
		return r.writeJSON(output)
	}
	return r.table(
		"ID\tPROJECT_ID\tPROJECT\tAMOUNT_MINOR\tCURRENCY\tPAID_AT\tPAID_FOR\tNOTE",
		[]string{fmt.Sprintf("%d\t%d\t%s\t%d\t%s\t%s\t%s\t%s", output.ID,
			output.ProjectID, output.ProjectName, output.AmountMinor,
			output.Currency, output.PaidAt, output.PaidForDate, output.Note)},
	)
}

func (r runner) createPayment(args []string) error {
	today := r.now().Format(dateLayout)
	flags := r.flags("payments", "create")
	projectID := flags.Int("project", 0, "project ID")
	amount := flags.Int("amount-minor", 0, "amount in minor units")
	currency := flags.String("currency", "", "three-letter currency code")
	paidAt := flags.String("paid-at", today, "payment date")
	paidFor := flags.String("paid-for", today, "date the payment covers")
	note := flags.String("note", "", "payment note")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if err := required(flags, "project", "amount-minor", "currency"); err != nil {
		return err
	}
	paidAtValue, err := parseDate(*paidAt)
	if err != nil {
		return err
	}
	paidForValue, err := parseDate(*paidFor)
	if err != nil {
		return err
	}
	id, err := r.stor.CreatePaymentID(r.ctx, storage.Payment{
		ProjectID: *projectID, AmountMinor: *amount, Currency: *currency,
		PaidAt: paidAtValue, PaidForDate: paidForValue, Note: *note,
	})
	if err != nil {
		return err
	}
	return r.status("created", "payment", id)
}

func (r runner) updatePayment(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: chankat payments update ID [options]")
	}
	id, err := parseID(args[0], "payment")
	if err != nil {
		return err
	}
	payment, err := r.stor.GetPayment(r.ctx, id)
	if err != nil {
		return err
	}
	flags := r.flags("payments", "update")
	projectID := flags.Int("project", payment.ProjectID, "project ID")
	amount := flags.Int("amount-minor", payment.AmountMinor, "amount in minor units")
	currency := flags.String("currency", payment.Currency, "three-letter currency code")
	paidAt := flags.String("paid-at", payment.PaidAt.Format(dateLayout), "payment date")
	paidFor := flags.String("paid-for", payment.PaidForDate.Format(dateLayout),
		"date the payment covers")
	note := flags.String("note", payment.Note, "payment note")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if flags.NFlag() == 0 {
		return fmt.Errorf("expected at least one update option")
	}
	paidAtValue, err := parseDate(*paidAt)
	if err != nil {
		return err
	}
	paidForValue, err := parseDate(*paidFor)
	if err != nil {
		return err
	}
	payment.ProjectID = *projectID
	payment.AmountMinor = *amount
	payment.Currency = *currency
	payment.PaidAt = paidAtValue
	payment.PaidForDate = paidForValue
	payment.Note = *note
	if err := r.stor.UpdatePayment(r.ctx, payment); err != nil {
		return err
	}
	return r.status("updated", "payment", id)
}

func (r runner) deletePayment(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: chankat payments delete ID")
	}
	id, err := parseID(args[0], "payment")
	if err != nil {
		return err
	}
	if err := r.stor.DeletePayment(r.ctx, id); err != nil {
		return err
	}
	return r.status("deleted", "payment", id)
}
