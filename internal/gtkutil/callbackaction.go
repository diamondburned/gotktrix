package gtkutil

import (
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// CallbackAction extends SimpleAction to provide idiomatic callback APIs.
type CallbackAction struct {
	*gio.SimpleAction
}

// NewCallbackAction creates a new CallbackAction.
func NewCallbackAction(name string) *CallbackAction {
	a := gio.NewSimpleAction(name, nil)
	return &CallbackAction{a}
}

// ActionFunc creates a CallbackActionFunc from a function.
func ActionFunc(name string, f func()) *CallbackAction {
	c := NewCallbackAction(name)
	c.OnActivate(f)
	return c
}

// OnActivate binds the given function callback to be called when the action is
// activated.
func (a *CallbackAction) OnActivate(f func()) {
	a.SimpleAction.Connect("activate", f)
}

// ActionData describes a CallbackAction's data.
type ActionData struct {
	Name string
	Func func()
}

// ActionGroup constructs an action group from the diven action data.
func ActionGroup(data ...gio.Actioner) *gio.SimpleActionGroup {
	group := gio.NewSimpleActionGroup()
	for _, data := range data {
		group.Insert(data)
	}
	return group
}

// BindActionMap binds the given map of actions (of key prefixed appropriately)
// to the given widget.
func BindActionMap(w gtk.Widgetter, prefix string, m map[string]func()) {
	group := gio.NewSimpleActionGroup()
	for k, v := range m {
		group.Insert(ActionFunc(k, v))
	}

	w.InsertActionGroup(prefix, group)
}

func NewCustomMenuItem(label, id string) *gio.MenuItem {
	item := gio.NewMenuItem(label, id)
	item.SetAttributeValue("custom", glib.NewVariantString(id))
	return item
}

// MenuPair creates a gtk.Menu out of the given menu pair. The returned Menu
// instance satisfies gio.MenuModeller. The first value of a pair should be the
// name.
func MenuPair(pairs [][2]string) *gio.Menu {
	menu := gio.NewMenu()
	for _, pair := range pairs {
		menu.Append(pair[0], pair[1])
	}
	return menu
}

// BindPopoverMenu binds the given widget to a popover menu to be displayed on
// right-clicking.
func BindPopoverMenu(w gtk.Widgetter, pos gtk.PositionType, pairs [][2]string) {
	BindRightClick(w, func() {
		popover := gtk.NewPopoverMenuFromModel(MenuPair(pairs))
		popover.SetPosition(pos)
		popover.SetParent(w)
		popover.Popup()
	})
}

// NewPopoverMenuFromPairs is a convenient function for NewPopoverMenuFromModel
// and MenuPairs.
func NewPopoverMenuFromPairs(pairs [][2]string) *gtk.PopoverMenu {
	return gtk.NewPopoverMenuFromModel(MenuPair(pairs))
}

// RadioData describes the data for the set of radio buttons created by
// NewRadioButtons.
type RadioData struct {
	Current int
	Options []string
}

// NewRadioButtons creates a new box of radio buttons.
func NewRadioButtons(d RadioData, f func(int)) gtk.Widgetter {
	b := gtk.NewBox(gtk.OrientationVertical, 0)
	b.AddCSSClass("radio-buttons")

	var first *gtk.CheckButton

	for i, opt := range d.Options {
		i := i

		radio := gtk.NewCheckButtonWithLabel(opt)
		radio.Connect("toggled", func() {
			if radio.Active() {
				f(i)
			}
		})

		if d.Current == i {
			radio.SetActive(true)
		}

		if first != nil {
			radio.SetGroup(first)
		} else {
			first = radio
		}

		b.Append(radio)
	}

	return b
}
