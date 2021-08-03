package gtkutil

import (
	"github.com/diamondburned/gotk4/pkg/gio/v2"
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

// BindPopoverMenu binds the given widget to a popover menu to be displayed on
// right-clicking.
func BindPopoverMenu(w gtk.Widgetter, pairs [][2]string) {
	BindRightClick(w, func() {
		menu := gio.NewMenu()
		for _, pair := range pairs {
			menu.Append(pair[0], pair[1])
		}

		popover := gtk.NewPopoverMenuFromModel(menu)
		popover.SetPosition(gtk.PosRight)
		popover.SetParent(w)
		popover.Popup()
	})
}
