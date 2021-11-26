package animations

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

const breathingChar = "●"

var breathingCSS = cssutil.Applier("animations-breathingdots", `
	@keyframes breathing {
		  0% { opacity: 0.66; }
		100% { opacity: 0.12; }
	}
	.animations-breathingdots label {
		animation: breathing 800ms infinite alternate;
	}
	.animations-breathingdots label:nth-child(1) {
		animation-delay: 000ms;
	}
	.animations-breathingdots label:nth-child(2) {
		animation-delay: 150ms;
	}
	.animations-breathingdots label:nth-child(3) {
		animation-delay: 300ms;
	}
`)

// NewBreathingDots creates a new breathing animation of 3 fading dots.
func NewBreathingDots() gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	c1 := gtk.NewLabel(breathingChar)
	c2 := gtk.NewLabel(breathingChar)
	c3 := gtk.NewLabel(breathingChar)

	box.Append(c1)
	box.Append(c2)
	box.Append(c3)

	breathingCSS(box)

	return box
}
