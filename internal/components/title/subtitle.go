// Package title contains title widgets.
package title

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
)

// Subtitle is a title widget with a subtitle on the next line.
type Subtitle struct {
	*gtk.Box
	Title    *gtk.Label
	Subtitle *gtk.Label
}

var subtitleCSS = cssutil.Applier("subtitle", `
	.subtitle {
		min-height: 42px;
		padding: 0;
	}
	.subtitle:not(:backdrop) {
		color: @theme_fg_color;
	}
	.subtitle-title {
		font-size: 1rem;
	}
	.subtitle-subtitle {
		font-size: 0.9rem;
		margin-top: -8px;
	}
`)

// NewSubtitle creates a new subtitle widget.
func NewSubtitle() *Subtitle {
	s := &Subtitle{}
	s.Title = gtk.NewLabel("")
	s.Title.SetVExpand(true)
	s.Title.SetEllipsize(pango.EllipsizeEnd)
	s.Title.AddCSSClass("subtitle-title")

	s.Subtitle = gtk.NewLabel("")
	s.Subtitle.SetVExpand(true)
	s.Subtitle.SetEllipsize(pango.EllipsizeEnd)
	s.Subtitle.AddCSSClass("subtitle-subtitle")

	s.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	s.Box.Append(s.Title)
	s.Box.Append(s.Subtitle)
	subtitleCSS(s)

	return s
}

// SetTitle sets the title.
func (s *Subtitle) SetTitle(title string) {
	s.Title.SetText(title)
}

// SetSubtitle sets the subtitle. If subtitle is empty, then it's hidden.
func (s *Subtitle) SetSubtitle(subtitle string) {
	s.Subtitle.SetText(subtitle)
	s.Subtitle.SetVisible(subtitle != "")
}

// SetXAlign sets the X align of the subtitles.
func (s *Subtitle) SetXAlign(xalign float32) {
	s.Title.SetXAlign(xalign)
	s.Subtitle.SetXAlign(xalign)
}
