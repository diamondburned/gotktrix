package section

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

var iconButtonCSS = cssutil.Applier("roomlist-iconbutton", `
	.roomlist-iconbutton {
		font-size: 0.9em;
		color: alpha(@theme_fg_color, 0.85);

		border: none;
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
		color: mix(@theme_selected_bg_color, @theme_fg_color, 0.35);
		background-color: alpha(@theme_fg_color, 0.1);
	}

	.roomlist-showmore {
		font-weight: initial;
		background: none;
		opacity: 0.6;
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

	button.ConnectToggled(func() {
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

const cannotMinify = -1

type minifyButton struct {
	iconButton
	ctx   context.Context
	nFunc func() int
}

func newMinifyButton(ctx context.Context, minify bool) *minifyButton {
	button := newIconButton("", minifyIconName(minify))
	button.SetActive(!minify)
	button.AddCSSClass("roomlist-showmore")

	return &minifyButton{
		*button,
		ctx,
		func() int { return 0 },
	}
}

func minifyIconName(minify bool) string {
	if minify {
		return "go-down-symbolic"
	}
	return "go-up-symbolic"
}

func (b *minifyButton) IsMinified() bool {
	return !b.Active()
}

func (b *minifyButton) SetMinified(minified bool) {
	b.ToggleButton.SetActive(!minified)
	b.Invalidate()
}

func (b *minifyButton) SetFunc(f func() (nHidden int)) {
	b.nFunc = f
}

func (b *minifyButton) Invalidate() {
	minified := b.IsMinified()
	nHidden := b.nFunc()

	if nHidden == cannotMinify {
		b.Hide()
		return
	}

	b.Show()

	if minified {
		b.label.SetLabel(locale.Sprintf(b.ctx, "Show %d more", nHidden))
	} else {
		b.label.SetLabel(locale.S(b.ctx, "Show less"))
	}
	b.icon.SetFromIconName(minifyIconName(minified))
}
