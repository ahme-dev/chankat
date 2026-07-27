package components

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type fixtureItem struct {
	id int
}

func (i fixtureItem) Title() string       { return "item" }
func (i fixtureItem) Description() string { return "" }
func (i fixtureItem) FilterValue() string { return "item" }

type detailedFixtureItem struct{}

func (detailedFixtureItem) Title() string       { return "title" }
func (detailedFixtureItem) Description() string { return "description" }
func (detailedFixtureItem) FilterValue() string { return "title description" }

func TestCRUDPage(t *testing.T) {
	form := func() *Form[fixtureItem] {
		return NewForm[fixtureItem](
			t.Context(),
			"fixture",
			huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("save"))),
			func(context.Context) error { return nil },
		)
	}
	var updatedID int
	page := NewPage(t.Context(), Config[fixtureItem]{
		Name: "fixtures",
		Load: func(context.Context) ([]fixtureItem, any, error) {
			return []fixtureItem{{id: 7}}, nil, nil
		},
		Create: func(any) (*Form[fixtureItem], error) {
			return form(), nil
		},
		Update: func(item fixtureItem, _ any) (*Form[fixtureItem], error) {
			updatedID = item.id
			return form(), nil
		},
		Delete: func(fixtureItem) *Form[fixtureItem] {
			return form()
		},
	})

	loaded := page.load()().(crudLoadedMsg[fixtureItem])
	page, _ = page.Update(loaded)

	t.Run("loads items", func(t *testing.T) {
		if page.loading || len(page.items) != 1 {
			t.Fatalf("got loading=%v items=%d", page.loading, len(page.items))
		}
		if page.list.FilterState() != list.Unfiltered {
			t.Fatal("list unexpectedly filtered")
		}
	})

	t.Run("opens create", func(t *testing.T) {
		updated, _ := page.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		if updated.form == nil {
			t.Fatal("create form was not opened")
		}
	})

	t.Run("escape closes form", func(t *testing.T) {
		updated, _ := page.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if updated.form != nil {
			t.Fatal("form remained open after escape")
		}
	})

	t.Run("opens selected item for update", func(t *testing.T) {
		updated, _ := page.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if updated.form == nil || updatedID != 7 {
			t.Fatalf("got form=%v updated ID=%d", updated.form != nil, updatedID)
		}
	})

	t.Run("opens delete confirmation", func(t *testing.T) {
		updated, _ := page.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
		if updated.form == nil {
			t.Fatal("delete form was not opened")
		}
	})

	t.Run("allows actions on filtered results", func(t *testing.T) {
		filtered := page
		filtered.list.SetFilterText("item")
		if !filtered.GlobalKeysEnabled() {
			t.Fatal("actions disabled after applying filter")
		}
		updated, _ := filtered.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if updated.form == nil {
			t.Fatal("update form was not opened for filtered item")
		}
	})

	t.Run("blocks actions while entering filter", func(t *testing.T) {
		filtering := page
		filtering.list.SetFilterState(list.Filtering)
		if filtering.GlobalKeysEnabled() {
			t.Fatal("actions enabled while entering filter")
		}
	})

	t.Run("resets filter", func(t *testing.T) {
		filtered := page
		filtered.list.SetFilterText("item")
		filtered.ResetFilter()
		if filtered.list.FilterState() != list.Unfiltered {
			t.Fatal("filter was not reset")
		}
	})
}

func TestCRUDItemText(t *testing.T) {
	item := fixtureItem{id: 1}
	if got := crudItemText(item); got != "item" {
		t.Fatalf("got %q, want item title", got)
	}

	want := "title\ndescription"
	if got := crudItemText(detailedFixtureItem{}); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
