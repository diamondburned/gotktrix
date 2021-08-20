package section

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

var iconButtonCSS = cssutil.Applier("roomlist-iconbutton", `
	.roomlist-iconbutton {
		font-size: 0.9em;
		color: alpha(@theme_fg_color, 0.85);

		padding: 2px;
		border-radius: 0;
	}
	.roomlist-iconbutton image {
		min-width: 32px;
		margin:  2px 6px;
		padding: 0;
	}

	.roomlist-expand {
		font-weight: bold;
	}
	.roomlist-expand:checked {
		color: @accent_color;
		background-color: alpha(@accent_fg_color, 0.1);
	}

	.roomlist-showmore {
		font-weight: initial;
		background: none;
	}
	.roomlist-showmore * {
		color: alpha(@theme_fg_color, 0.75);	
	}
`)

type iconButton struct {
	*gtk.ToggleButton
	icon  *gtk.Image
	label *gtk.Label
}

func newIconButton(name, icon string) *iconButton {
	arrow := gtk.NewImageFromIconName(icon)
	arrow.SetPixelSize(16)

	label := gtk.NewLabel(name)
	label.SetHExpand(true)
	label.SetXAlign(0)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(arrow)
	box.Append(label)

	button := gtk.NewToggleButton()
	button.SetChild(box)
	iconButtonCSS(button)

	return &iconButton{
		ToggleButton: button,
		icon:         arrow,
		label:        label,
	}
}

func newRevealButton(rev *gtk.Revealer, name string) *iconButton {
	button := newIconButton(name, revealIconName(rev.RevealChild()))
	button.SetActive(rev.RevealChild())
	button.AddCSSClass("roomlist-expand")

	icon := button.icon

	button.Connect("toggled", func(button *gtk.ToggleButton) {
		reveal := button.Active()
		rev.SetRevealChild(reveal)
		icon.SetFromIconName(revealIconName(reveal))
	})

	return button
}

func revealIconName(rev bool) string {
	if rev {
		return "go-down-symbolic"
	}
	return "go-next-symbolic"
}

type minifyButton struct {
	iconButton
	labelFn func(bool) string
}

func newMinifyButton(minify bool) *minifyButton {
	button := newIconButton("", minifyIconName(minify))
	button.SetActive(!minify)
	button.AddCSSClass("roomlist-showmore")

	return &minifyButton{
		*button,
		nil,
	}
}

func minifyIconName(minify bool) string {
	if minify {
		return "go-down-symbolic"
	}
	return "go-up-symbolic"
}

func (b *minifyButton) Minify() bool {
	return !b.Active()
}

func (b *minifyButton) OnToggled(labelFn func(bool) string) {
	b.labelFn = labelFn
	b.Invalidate()

	icon := b.icon
	label := b.label
	label.SetLabel(labelFn(!b.Active()))

	b.Connect("toggled", func(button *gtk.ToggleButton) {
		minify := !button.Active()
		icon.SetFromIconName(minifyIconName(minify))
		label.SetLabel(labelFn(minify))
	})
}

func (b *minifyButton) Invalidate() {
	b.label.SetLabel(b.labelFn(b.Minify()))
}
