package progress

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

var barCSS = cssutil.Applier("progress-bar", `
	.progress-error {
		color: @error_color;
	}
	.progress-error trough {
		background-color: alpha(@error_color, 0.2);
	}
	.progress-error progress {
		background-color: @error_color;
	}
`)

// Bar describes a progress bar.
type Bar struct {
	*gtk.ProgressBar
	label func(n, max int64) string
	n     int64
	maxi  int64
	maxf  float64
	error bool
}

// NewBar creates a new progress bar.
func NewBar() *Bar {
	b := Bar{}
	b.ProgressBar = gtk.NewProgressBar()
	barCSS(b)
	return &b
}

// SetMax sets the max value of the bar.
func (b *Bar) SetMax(max int64) {
	b.maxi = max
	b.maxf = float64(max)
}

// Add adds the given n value.
func (b *Bar) Add(n int64) {
	b.Set(b.n + n)
}

// Set sets the given n value.
func (b *Bar) Set(n int64) {
	b.n = n

	if b.error {
		b.RemoveCSSClass("progress-error")
		b.error = false
	}

	if b.maxi == 0 {
		b.ProgressBar.Pulse()
		return
	}

	b.ProgressBar.SetFraction(float64(b.n) / b.maxf)

	if b.label != nil {
		b.ProgressBar.SetText(b.label(b.n, b.maxi))
	}
}

// Error changes bar to indicate an error.
func (b *Bar) Error(err error) {
	if !b.error {
		b.AddCSSClass("progress-error")
		b.error = true
	}

	b.SetText("Error: " + err.Error())
}

// SetLabelFunc sets the function to render the progress bar label. It only does
// something if Max is set to a value. This method must only be called before
// Add is called, otherwise the state is invalid.
func (b *Bar) SetLabelFunc(labelFn func(n, max int64) string) {
	b.label = labelFn
	b.SetText(labelFn(b.n, b.maxi))
	b.SetShowText(labelFn != nil)
}
