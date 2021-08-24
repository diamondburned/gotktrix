package autocomplete

import (
	"context"
	"unicode"
	"unicode/utf8"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix/indexer"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// SelectedData wraps around a Data to provide additional metadata that could be
// useful for the user.
type SelectedData struct {
	// Cursor is the iterator that sits at the current cursor location at the
	// time this data was found.
	Cursor *gtk.TextIter
	// Data is the selected entry's data.
	Data Data
}

// SelectedFunc is the callback type that is called when the user has selected
// an entry inside the autocompleter. If the callback returns true, then the
// autocompleter closes itself; otherwise, it does nothing.
type SelectedFunc func(SelectedData) bool

// Autocompleter is the autocompleter instance.
type Autocompleter struct {
	tview  *gtk.TextView
	buffer *gtk.TextBuffer
	cursor *gtk.TextIter

	onSelect SelectedFunc

	popover  *gtk.Popover
	listBox  *gtk.ListBox
	listRows []row

	searchers map[rune]Searcher
}

type row struct {
	*gtk.ListBoxRow
	data Data
}

var _ = cssutil.WriteCSS(`
	.autocomplete-row {
		padding: 2px 6px;
	}
	.autocomplete-row label:nth-child(1) {
		margin-bottom: -2px;
	}
	.autocomplete-row label:nth-child(2) {
		margin-top: -2px;
	}
`)

// AutocompleterWidth is the minimum width of the popped up autocompleter.
const AutocompleterWidth = 250

// New creates a new instance of autocompleter.
func New(text *gtk.TextView, f SelectedFunc) *Autocompleter {
	list := gtk.NewListBox()
	list.AddCSSClass("autocomplete-list")
	list.SetActivateOnSingleClick(true)
	list.SetSelectionMode(gtk.SelectionBrowse)

	viewport := gtk.NewViewport(nil, nil)
	viewport.SetChild(list)
	viewport.SetVAlign(gtk.AlignStart)
	viewport.SetVExpand(true)
	viewport.SetVScrollPolicy(gtk.ScrollNatural)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(viewport)

	popover := gtk.NewPopover()
	popover.AddCSSClass("autocomplete-popover")
	popover.SetSizeRequest(AutocompleterWidth, 250)
	popover.SetParent(text)
	popover.SetChild(scroll)
	popover.SetPosition(gtk.PosTop)
	popover.SetAutohide(false)
	popover.Hide()

	ac := Autocompleter{
		tview:     text,
		buffer:    text.Buffer(),
		onSelect:  f,
		popover:   popover,
		listBox:   list,
		listRows:  make([]row, 0, indexer.QuerySize),
		searchers: make(map[rune]Searcher),
	}

	list.Connect("row-activated", func(list *gtk.ListBox, row *gtk.ListBoxRow) {
		ac.selectRow(row)
	})

	return &ac
}

// Use registers the given searcher instance into the autocompleter.
func (a *Autocompleter) Use(searchers ...Searcher) {
	for _, s := range searchers {
		a.searchers[s.Rune()] = s
	}
}

func popRune(s string) (rune, string) {
	r, sz := utf8.DecodeRuneInString(s)
	return r, s[sz:]
}

// Autocomplete updates the Autocompleter popover to show what the internal
// input buffer has.
func (a *Autocompleter) Autocomplete(ctx context.Context) {
	a.clear()

	cursor := a.buffer.IterAtOffset(a.buffer.ObjectProperty("cursor-position").(int))
	a.cursor = &cursor

	var searcher Searcher

	start := a.cursor.Copy()

	if !start.BackwardFindChar(func(ch uint32) bool {
		r := rune(ch)
		if unicode.IsSpace(r) {
			return false
		}

		var ok bool
		searcher, ok = a.searchers[r]
		return ok
	}, nil) || searcher == nil {
		a.hide()
		return
	}

	text := a.buffer.Text(start, a.cursor, false)
	first, text := popRune(text)

	searcher, ok := a.searchers[first]
	if !ok {
		a.hide()
		return
	}

	results := searcher.Search(ctx, text)
	if len(results) == 0 {
		a.hide()
		return
	}

	for _, result := range results {
		r := row{
			ListBoxRow: result.Row(),
			data:       result,
		}

		r.AddCSSClass("autocomplete-row")

		a.listBox.Append(r.ListBoxRow)
		a.listRows = append(a.listRows, r)
	}

	a.listBox.SelectRow(a.listRows[0].ListBoxRow)
	a.show()
}

// IsVisible returns true if the popover is currently visible.
func (a *Autocompleter) IsVisible() bool {
	return len(a.listRows) > 0 && a.popover.IsVisible()
}

// Select selects the current Autocompleter entry.
func (a *Autocompleter) Select() {
	a.selectRow(a.listBox.SelectedRow())
}

func (a *Autocompleter) selectRow(row *gtk.ListBoxRow) {
	if row == nil {
		a.Clear()
		return
	}

	data := SelectedData{
		Cursor: a.cursor,
		Data:   a.listRows[row.Index()].data,
	}

	if a.onSelect(data) {
		a.buffer.Insert(data.Cursor, " ", 1)
		a.Clear()
		return
	}
}

// Clear clears the Autocompleter and hides it.
func (a *Autocompleter) Clear() {
	a.clear()
	a.hide()
}

func (a *Autocompleter) hide() {
	a.popover.Popdown()
}

func (a *Autocompleter) show() {
	rect := a.tview.IterLocation(a.cursor)
	x, y := a.tview.BufferToWindowCoords(gtk.TextWindowWidget, rect.X(), rect.Y())

	ptTo := gdk.NewRectangle(x, y, 1, 1)
	a.popover.SetPointingTo(&ptTo)
	a.popover.Popup()
}

func (a *Autocompleter) clear() {
	for i, r := range a.listRows {
		a.listBox.Remove(r.ListBoxRow)
		a.listRows[i] = row{}
	}
	a.listRows = a.listRows[:0]
}

func (a *Autocompleter) MoveUp() bool   { return a.move(false) }
func (a *Autocompleter) MoveDown() bool { return a.move(true) }

func (a *Autocompleter) move(down bool) bool {
	if len(a.listRows) == 0 {
		return false
	}

	row := a.listBox.SelectedRow()
	if row == nil {
		a.listBox.SelectRow(a.listRows[0].ListBoxRow)
		return true
	}

	ix := row.Index()
	if down {
		ix++
		if ix == len(a.listRows) {
			ix = 0
		}
	} else {
		ix--
		if ix == -1 {
			ix = len(a.listRows) - 1
		}
	}

	a.listBox.SelectRow(a.listRows[ix].ListBoxRow)
	return true
}
