package gtkutil

import "github.com/diamondburned/gotk4/pkg/gio/v2"

// CallbackAction extends SimpleAction to provide idiomatic callback APIs.
type CallbackAction struct {
	*gio.SimpleAction
}

// NewCallbackAction creates a new CallbackAction.
func NewCallbackAction(name string) *CallbackAction {
	a := gio.NewSimpleAction(name, nil)
	return &CallbackAction{a}
}

// CallbackActionFunc creates a CallbackActionFunc from a function.
func CallbackActionFunc(name string, f func()) *CallbackAction {
	c := NewCallbackAction(name)
	c.OnActivate(f)
	return c
}

// OnActivate binds the given function callback to be called when the action is
// activated.
func (a *CallbackAction) OnActivate(f func()) {
	a.SimpleAction.Connect("activate", f)
}
