package tui

import (
	"fmt"

	"chansat/internal/storage"

	"github.com/charmbracelet/bubbles/list"
)

type rateItem struct {
	storage.Rate
}

func (r rateItem) Title() string {
	return r.Name
}

func (r rateItem) Description() string {
	return fmt.Sprintf("%d %s", r.AmountMinor, r.Currency)
}

func (r rateItem) FilterValue() string {
	return r.Name
}

func rateItems(rates []storage.Rate) []list.Item {
	items := make([]list.Item, len(rates))
	for i, rate := range rates {
		items[i] = rateItem{Rate: rate}
	}
	return items
}
