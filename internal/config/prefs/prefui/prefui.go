package prefui

import (
	"context"
	"log"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/config/prefs"
)

// View is a widget that lists all known preferences.
type View struct {
	*adaptive.Fold
	left  *gtk.StackSwitcher
	right *gtk.Stack
}

// NewView creates a new preference UI.
func NewView(ctx context.Context) *View {
	stack := gtk.NewStack()
	stack.SetHExpand(true)
	stack.AddCSSClass("prefui-stack")

	sections := prefs.ListProperties(ctx)
	for _, section := range sections {
		page := newPrefPage(section)
		stack.AddTitled(page, section.Name, section.Name)
	}

	switcher := gtk.NewStackSwitcher()
	switcher.SetStack(stack)

	fold := adaptive.NewFold(gtk.PosLeft)
	fold.SetFoldWidth(150)
	fold.SetFoldThreshold(450)
	fold.SetSideChild(switcher)
	fold.SetChild(stack)

	return &View{
		Fold:  fold,
		left:  switcher,
		right: stack,
	}
}

func newPrefPage(section prefs.Section) gtk.Widgetter {
	panic("TODO")
}

func bindPropWidget(p prefs.Prop, w gtk.Widgetter, changed string, set, publish func()) {
	var paused bool
	w.Connect(changed, func() {
		if !paused {
			publish()
		}
	})
	prefs.Connect(p, w, func() {
		paused = true
		set()
		paused = false
	})
}

func newPropWidget(p prefs.Prop) gtk.Widgetter {
	switch p := p.(type) {
	case *prefs.Bool:
		sw := gtk.NewSwitch()
		sw.AddCSSClass("prefui-prop")
		sw.SetHAlign(gtk.AlignEnd)
		bindPropWidget(p, sw, "notify::active",
			func() { sw.SetActive(p.Value()) },
			func() { p.Publish(sw.Active()) },
		)
		return sw
	case *prefs.Int:
		scale := gtk.NewSpinButtonWithRange(float64(p.Min), float64(p.Max), 1)
		scale.AddCSSClass("prefui-prop")
		scale.SetHAlign(gtk.AlignEnd)
		bindPropWidget(p, scale, "changed",
			func() { scale.SetValue(float64(p.Value())) },
			func() { p.Publish(scale.ValueAsInt()) },
		)
		return scale
	case *prefs.String:
		entry := gtk.NewEntry()
		entry.AddCSSClass("prefui-prop")
		entry.SetHAlign(gtk.AlignEnd)
		entry.SetSizeRequest(100, -1)
		entry.SetPlaceholderText(p.Placeholder)
		bindPropWidget(p, entry, "changed",
			func() { entry.SetText(p.Value()) },
			func() { p.Publish(entry.Text()) },
		)
		return entry
	// case *prefs.EnumList:
	// 	combo := gtk.NewComboBoxText()
	// 	combo.AddCSSClass("prefui-prop")
	// 	for _, str := range p.PossibleValueStrings() {
	// 		combo.Append(str, str)
	// 	}
	// 	bindPropWidget(p, combo, "changed",
	// 		func() { combo.SetActiveID(p.Value().String()) },
	// 		func() { p.Publish(combo.ActiveID()) },
	// 	)
	// 	return combo
	default:
		log.Panicf("unknown property type %T", p)
		return nil
	}
}
