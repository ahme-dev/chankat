package components

import (
	"context"
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type crudOperation int

const (
	crudCreate crudOperation = iota
	crudUpdate
	crudDelete
)

type crudLoadedMsg[T list.Item] struct {
	items []T
	meta  any
}

type crudLoadFailedMsg[T list.Item] struct {
	err error
}

type crudSavedMsg[T list.Item] struct{}

type crudSaveFailedMsg[T list.Item] struct {
	err error
}

type crudCopiedMsg[T list.Item] struct {
	err error
}

type Config[T list.Item] struct {
	Name      string
	Load      func(context.Context) ([]T, any, error)
	Create    func(any) (*Form[T], error)
	Update    func(T, any) (*Form[T], error)
	Delete    func(T) *Form[T]
	AfterSave func() tea.Cmd
	Embedded  bool
}

type Page[T list.Item] struct {
	ctx     context.Context
	config  Config[T]
	list    list.Model
	items   []T
	meta    any
	form    *Form[T]
	err     error
	loading bool
}

func NewPage[T list.Item](ctx context.Context, config Config[T]) Page[T] {
	delegate := NewListDelegate()
	items := list.New(nil, delegate, 0, 0)
	items.SetShowTitle(false)
	items.Styles.FilterCursor = items.Styles.FilterCursor.Foreground(lipgloss.Color("9"))
	if config.Embedded {
		items.SetShowStatusBar(false)
	}
	return Page[T]{
		ctx: ctx, config: config, list: items, loading: true,
	}
}

func NewListDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	red := lipgloss.Color("9")
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(red).
		BorderLeftForeground(red)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(red).
		BorderLeftForeground(red)
	return delegate
}

func NewInactiveListDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	delegate.Styles.SelectedTitle = delegate.Styles.NormalTitle
	delegate.Styles.SelectedDesc = delegate.Styles.NormalDesc
	return delegate
}

func (m Page[T]) Init() tea.Cmd {
	return m.load()
}

func (m Page[T]) Update(msg tea.Msg) (Page[T], tea.Cmd) {
	if m.form != nil {
		switch msg := msg.(type) {
		case crudSavedMsg[T]:
			m.form = nil
			m.loading = true
			var afterSave tea.Cmd
			if m.config.AfterSave != nil {
				afterSave = m.config.AfterSave()
			}
			return m, tea.Batch(m.load(), afterSave)
		case crudSaveFailedMsg[T]:
			m.form.fail(msg.err)
			return m, nil
		default:
			cmd := m.form.Update(msg)
			if m.form.aborted() {
				m.form = nil
				return m, nil
			}
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case crudLoadedMsg[T]:
		m.items = msg.items
		m.meta = msg.meta
		m.loading = false
		m.err = nil
		listItems := make([]list.Item, len(msg.items))
		for i := range msg.items {
			listItems[i] = msg.items[i]
		}
		return m, m.list.SetItems(listItems)
	case crudLoadFailedMsg[T]:
		m.loading = false
		m.err = msg.err
	case crudCopiedMsg[T]:
		status := "Copied."
		if msg.err != nil {
			status = "Copy failed: " + msg.err.Error()
		}
		return m, m.list.NewStatusMessage(status)
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "n":
			return m.open(crudCreate)
		case "e", "enter":
			return m.open(crudUpdate)
		case "x", "delete":
			return m.open(crudDelete)
		case "c":
			item, ok := m.selected()
			if !ok {
				return m, nil
			}
			return m, copyCRUDItem(item)
		case "r":
			m.loading = true
			return m, m.load()
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func copyCRUDItem[T list.Item](item T) tea.Cmd {
	return func() tea.Msg {
		return crudCopiedMsg[T]{err: clipboard.WriteAll(crudItemText(item))}
	}
}

func crudItemText(item list.Item) string {
	if item, ok := item.(list.DefaultItem); ok {
		description := item.Description()
		if description != "" {
			return item.Title() + "\n" + description
		}
		return item.Title()
	}
	return item.FilterValue()
}

func (m Page[T]) View() string {
	if m.form != nil {
		return m.form.View()
	}
	if m.loading {
		return "Loading " + m.config.Name + "..."
	}
	if m.err != nil {
		return "Error: " + m.err.Error() + ". Press 'r' to retry."
	}
	view := m.list.View()
	if m.config.Embedded && m.list.FilterState() != list.Filtering {
		view = strings.TrimPrefix(view, "\n")
	}
	return view
}

func (m Page[T]) FormActive() bool {
	return m.form != nil
}

func (m Page[T]) GlobalKeysEnabled() bool {
	return m.form == nil && m.list.FilterState() != list.Filtering
}

func (m Page[T]) Actions() string {
	return "[/] search  [n] new  [e/enter] edit  [x/delete] delete  [c] copy"
}

func (m *Page[T]) SetSize(width, height int) {
	m.list.SetSize(width, height)
}

func (m *Page[T]) SetItems(items []T) tea.Cmd {
	m.items = items
	m.loading = false
	listItems := make([]list.Item, len(items))
	for i := range items {
		listItems[i] = items[i]
	}
	return m.list.SetItems(listItems)
}

func (m *Page[T]) SetFocused(focused bool) {
	if focused {
		m.list.SetDelegate(NewListDelegate())
	} else {
		m.list.SetDelegate(NewInactiveListDelegate())
	}
}

func (m Page[T]) Index() int {
	return m.list.Index()
}

func (m *Page[T]) Select(index int) {
	m.list.Select(index)
}

func (m *Page[T]) ResetFilter() {
	m.list.ResetFilter()
}

func (m Page[T]) VisibleCount() int {
	return len(m.list.VisibleItems())
}

func (m Page[T]) Selected() (T, bool) {
	var zero T
	item := m.list.SelectedItem()
	if item == nil {
		return zero, false
	}
	selected, ok := item.(T)
	return selected, ok
}

func (m *Page[T]) Reload() tea.Cmd {
	m.loading = true
	return m.load()
}

func (m Page[T]) OpenForm(form *Form[T]) (Page[T], tea.Cmd) {
	m.form = form
	return m, form.Init()
}

func (m Page[T]) load() tea.Cmd {
	return func() tea.Msg {
		items, meta, err := m.config.Load(m.ctx)
		if err != nil {
			return crudLoadFailedMsg[T]{err: err}
		}
		return crudLoadedMsg[T]{items: items, meta: meta}
	}
}

func (m Page[T]) open(operation crudOperation) (Page[T], tea.Cmd) {
	var (
		form *Form[T]
		err  error
	)
	switch operation {
	case crudCreate:
		form, err = m.config.Create(m.meta)
	case crudUpdate, crudDelete:
		item, ok := m.selected()
		if !ok {
			return m, nil
		}
		if operation == crudUpdate {
			form, err = m.config.Update(item, m.meta)
		} else {
			form = m.config.Delete(item)
		}
	}
	if err != nil {
		m.err = err
		return m, nil
	}
	m.form = form
	return m, form.Init()
}

func (m Page[T]) selected() (T, bool) {
	return m.Selected()
}

type Form[T list.Item] struct {
	title     string
	form      *huh.Form
	save      func(context.Context) error
	ctx       context.Context
	saving    bool
	cancelled bool
	err       error
}

func NewForm[T list.Item](
	ctx context.Context,
	title string,
	form *huh.Form,
	save func(context.Context) error,
) *Form[T] {
	return &Form[T]{ctx: ctx, title: title, form: form, save: save}
}

func NewDeleteForm[T list.Item](
	ctx context.Context,
	name string,
	save func(context.Context) error,
) *Form[T] {
	confirmed := false
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Delete " + name + "?").
			Affirmative("Delete").
			Negative("Cancel").
			Value(&confirmed),
	))
	return NewForm[T](ctx, "delete", form, func(ctx context.Context) error {
		if !confirmed {
			return nil
		}
		return save(ctx)
	})
}

func (m *Form[T]) Init() tea.Cmd {
	return m.form.Init()
}

func (m *Form[T]) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" && !m.saving {
		m.cancelled = true
		return nil
	}
	if m.err != nil {
		return nil
	}
	if m.saving {
		return nil
	}
	updated, cmd := m.form.Update(msg)
	m.form = updated.(*huh.Form)
	if m.form.State == huh.StateAborted {
		m.cancelled = true
		return nil
	}
	if m.form.State != huh.StateCompleted {
		return cmd
	}
	m.saving = true
	return func() tea.Msg {
		if err := m.save(m.ctx); err != nil {
			return crudSaveFailedMsg[T]{err: err}
		}
		return crudSavedMsg[T]{}
	}
}

func (m *Form[T]) View() string {
	if m.err != nil {
		return fmt.Sprintf("%s\n\nError: %v\n\n[esc] back", m.title, m.err)
	}
	if m.saving {
		return m.title + "\n\nSaving..."
	}
	return m.title + "\n\n" + m.form.View() + "\n\n[esc] back"
}

func (m *Form[T]) fail(err error) {
	m.saving = false
	m.err = err
}

func (m *Form[T]) aborted() bool {
	return m.cancelled
}
