// Package actionbutton contains GTK4 buttons that are layouted differently.
package actionbutton

import (
	"log"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Button is an action button that contains an icon and a label.
type Button struct {
	*gtk.Button
	Icon  *gtk.Image
	Label *gtk.Label
}

// NewButton creates a new button. The icon's position is determined by the
// given position type.
func NewButton(label, icon string, pos gtk.PositionType) *Button {
	img := gtk.NewImageFromIconName(icon)
	img.SetIconSize(gtk.IconSizeNormal)
	img.SetMarginBottom(1)

	lbl := gtk.NewLabel(label)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)

	switch pos {
	case gtk.PosLeft:
		img.SetMarginEnd(4)
		box.Append(img)
		box.Append(lbl)
	case gtk.PosRight:
		img.SetMarginStart(4)
		box.Append(lbl)
		box.Append(img)
	default:
		log.Panicf("unknown GTK position %v", pos.String())
	}

	button := gtk.NewButton()
	button.SetChild(box)

	return &Button{
		Button: button,
		Icon:   img,
		Label:  lbl,
	}
}
