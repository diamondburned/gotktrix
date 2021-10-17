package gtkutil

import (
	"log"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
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
	if f != nil {
		a.SimpleAction.Connect("activate", f)
	}
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
		group.AddAction(data)
	}
	return group
}

// BindActionMap binds the given map of actions (of key prefixed appropriately)
// to the given widget.
func BindActionMap(w gtk.Widgetter, prefix string, m map[string]func()) {
	group := gio.NewSimpleActionGroup()
	for k, v := range m {
		group.AddAction(ActionFunc(k, v))
	}

	w.InsertActionGroup(prefix, group)
}

func NewCustomMenuItem(label, id string) *gio.MenuItem {
	item := gio.NewMenuItem(label, id)
	setCustomMenuItem(item, id)
	return item
}

func setCustomMenuItem(item *gio.MenuItem, id string) {
	item.SetAttributeValue("custom", glib.NewVariantString(id))
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

// PopoverWidth is the default popover width.
const PopoverWidth = 150

// BindPopoverMenu binds the given widget to a popover menu to be displayed on
// right-clicking.
func BindPopoverMenu(w gtk.Widgetter, pos gtk.PositionType, pairs [][2]string) {
	BindRightClick(w, func() { ShowPopoverMenu(w, pos, pairs) })
}

// ShowPopoverMenu is like ShowPopoverMenuCustom but uses a regular string pair
// list.
func ShowPopoverMenu(w gtk.Widgetter, pos gtk.PositionType, pairs [][2]string) *gtk.PopoverMenu {
	p := NewPopoverMenu(w, pos, pairs)
	p.Popup()
	return p
}

// NewPopoverMenu creats a new Popover menu.
func NewPopoverMenu(w gtk.Widgetter, pos gtk.PositionType, pairs [][2]string) *gtk.PopoverMenu {
	popover := gtk.NewPopoverMenuFromModel(MenuPair(pairs))
	popover.SetMnemonicsVisible(true)
	popover.SetSizeRequest(PopoverWidth, -1)
	popover.SetPosition(pos)
	popover.SetParent(w)
	return popover
}

// PopoverMenuItem defines a popover menu item constructed from one of the
// constructors.
type PopoverMenuItem interface {
	menu()
}

type popoverMenuItem struct {
	label  string
	action string
	icon   string
	widget gtk.Widgetter
}

func (p popoverMenuItem) menu() {}

// MenuItem creates a simple popover menu item. If action is empty, then the
// item is disabled; if action is "---", then a new section is created.
func MenuItem(label, action string, ands ...bool) PopoverMenuItem {
	for _, and := range ands {
		if !and {
			return nil
		}
	}

	return popoverMenuItem{
		label:  label,
		action: action,
	}
}

// MenuItemIcon is an icon variant of MenuItem.
func MenuItemIcon(label, action, icon string) PopoverMenuItem {
	return popoverMenuItem{
		label:  label,
		action: action,
		icon:   icon,
	}
}

// MenuWidget creates a new menu item that contains a widget.
func MenuWidget(action string, w gtk.Widgetter) PopoverMenuItem {
	return popoverMenuItem{
		action: action,
		widget: w,
	}
}

// MenuSeparator creates a new menu separator.
func MenuSeparator(label string) PopoverMenuItem {
	return popoverMenuItem{
		label:  label,
		action: "---",
	}
}

type submenu struct {
	label string
	items []PopoverMenuItem
}

func (p submenu) menu() {}

// Submenu creates a popover menu item that is a submenu.
func Submenu(label string, sub []PopoverMenuItem) PopoverMenuItem {
	return submenu{
		label: label,
		items: sub,
	}
}

// BindPopoverMenuCustom works similarly to BindPopoverMenu, but the value type
// can be more than just an action string. The key must be a string.
func BindPopoverMenuCustom(w gtk.Widgetter, pos gtk.PositionType, pairs []PopoverMenuItem) {
	BindRightClickAt(w, func(x, y float64) {
		popover := PopoverMenuCustom(w, pos, pairs)
		if popover == nil {
			return
		}

		at := gdk.NewRectangle(int(x), int(y), 0, 0)
		popover.SetPointingTo(&at)
		popover.Popup()
	})
}

// BindPopoverMenuLazy is similarl to BindPopoverMenuCustom, except the menu
// items are lazily created.
func BindPopoverMenuLazy(w gtk.Widgetter, pos gtk.PositionType, pairsFn func() []PopoverMenuItem) {
	BindRightClick(w, func() { ShowPopoverMenuCustom(w, pos, pairsFn()) })
}

// CustomMenu returns a new Menu from the given popover menu items. All menu
// items that have widgets are ignored.
func CustomMenu(items []PopoverMenuItem) *gio.Menu {
	menu := gio.NewMenu()
	addMenuItems(menu, items, nil)
	return menu
}

func addMenuItems(menu *gio.Menu, items []PopoverMenuItem, widgets map[string]gtk.Widgetter) int {
	section := menu
	var added int

	for _, item := range items {
		if item == nil {
			continue
		}

		switch item := item.(type) {
		case popoverMenuItem:
			if item.widget != nil && widgets == nil {
				// No widgets supported; skip this menu item.
				continue
			}

			if item.action == "---" {
				section = gio.NewMenu()
				menu.AppendSection(item.label, section)
				continue
			}

			menu := gio.NewMenuItem(item.label, item.action)
			if item.icon != "" {
				menu.SetIcon(gio.NewThemedIcon(item.icon))
			}
			if item.widget != nil {
				widgets[item.action] = item.widget
				setCustomMenuItem(menu, item.action)
			}
			added++
			section.AppendItem(menu)
		case submenu:
			sub := gio.NewMenu()
			if addMenuItems(sub, item.items, widgets) > 0 {
				added++
				section.AppendSubmenu(item.label, sub)
			}
		default:
			log.Panicf("unknown menu item type %T", item)
		}
	}

	return added
}

// ShowPopoverMenuCustom is like BindPopoverMenuCustom, but it does not bind a
// handler. This is useful if the caller does not want pairs to be in memory all
// the time. If any of the menus cannot be added in, then false is returned, and
// the popover isn't shown.
func ShowPopoverMenuCustom(w gtk.Widgetter, pos gtk.PositionType, items []PopoverMenuItem) bool {
	popover := PopoverMenuCustom(w, pos, items)
	if popover == nil {
		return false
	}

	popover.Popup()
	return true
}

// PopoverMenuCustom creates a new Popover containing the given items.
func PopoverMenuCustom(
	w gtk.Widgetter, pos gtk.PositionType, items []PopoverMenuItem) *gtk.PopoverMenu {

	menu := gio.NewMenu()
	widgets := make(map[string]gtk.Widgetter)

	addMenuItems(menu, items, widgets)

	popover := gtk.NewPopoverMenuFromModel(menu)
	popover.SetAutohide(true)
	popover.SetCascadePopdown(false)
	popover.SetSizeRequest(PopoverWidth, -1)
	popover.SetPosition(pos)
	popover.SetParent(w)

	for action, widget := range widgets {
		if !popover.AddChild(widget, action) {
			return nil
		}
	}

	return popover
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
