// Package mauthor handles rendering Matrix room members' names.
package mauthor

import (
	"fmt"
	"html"
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/pronouns"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
)

type markupOpts struct {
	hasher    ColorHasher
	at        bool
	minimal   bool
	multiline bool
}

// MarkupMod is a function type that Markup can take multiples of. It
// changes subtle behaviors of the Markup function, such as the color hasher
// used.
type MarkupMod func(opts *markupOpts)

// WithMinimal renders the markup without additional information, such as
// pronouns.
func WithMinimal() MarkupMod {
	return func(opts *markupOpts) {
		opts.minimal = true
	}
}

// WithMultiline renders the markup in multiple lines.
func WithMultiline() MarkupMod {
	return func(opts *markupOpts) {
		opts.multiline = true
	}
}

// WithColorHasher uses the given color hasher.
func WithColorHasher(hasher ColorHasher) MarkupMod {
	return func(opts *markupOpts) {
		opts.hasher = hasher
	}
}

// WithMention makes the renderer prefix an at ("@") symbol.
func WithMention() MarkupMod {
	return func(opts *markupOpts) {
		opts.at = true
	}
}

// WithWidgetColor determines the best hasher from the given widget. The caller
// should beware to call this function in the main thread to not cause a race
// condition.
func WithWidgetColor(w gtk.Widgetter) MarkupMod {
	if markuputil.IsDarkTheme(w) {
		return WithColorHasher(LightColorHasher)
	} else {
		return WithColorHasher(DarkColorHasher)
	}
}

func mkopts(mods []MarkupMod) markupOpts {
	opts := markupOpts{
		hasher: DefaultColorHasher(),
	}

	for _, mod := range mods {
		mod(&opts)
	}

	return opts
}

// Markup renders the markup string for the given user inside the given room.
// The markup format follows the Pango markup format.
//
// If the given room ID string is empty, then certain information are skipped.
// If it's non-empty, then the state will be used to fetch additional
// information.
func Markup(c *gotktrix.Client, rID matrix.RoomID, uID matrix.UserID, mods ...MarkupMod) string {
	// TODO: maybe bridge role colors?

	opts := mkopts(mods)

	name, _, _ := uID.Parse()
	var ambiguous bool

	if rID != "" {
		n, err := c.MemberName(rID, uID)
		if err == nil {
			name = n.Name
			ambiguous = n.Ambiguous
		}
	}

	if opts.at {
		name = "@" + name
	}

	color := opts.hasher.Hash(string(uID))

	b := strings.Builder{}
	b.Grow(512)
	b.WriteString(fmt.Sprintf(
		`<span color="%s">%s</span>`,
		RGBHex(color), html.EscapeString(name),
	))

	if opts.minimal {
		return b.String()
	}

	if pronoun := pronouns.UserPronouns(c, rID, uID).Pronoun(); pronoun != "" {
		if opts.multiline {
			b.WriteByte('\n')
		} else {
			b.WriteByte(' ')
		}
		b.WriteString(fmt.Sprintf(
			`<span fgalpha="90%%" size="small">(%s)</span>`,
			html.EscapeString(string(pronoun)),
		))
	}

	if ambiguous {
		if opts.multiline {
			b.WriteByte('\n')
		} else {
			b.WriteByte(' ')
		}
		b.WriteString(fmt.Sprintf(
			` <span fgalpha="80%%" size="small">(%s)</span>`,
			html.EscapeString(string(uID)),
		))
	}

	return b.String()
}

// Text renders the author's name into a rich text buffer. Tje written string is
// always minimal. The inserted tags have the "_mauthor" prefix.
func Text(c *gotktrix.Client, iter *gtk.TextIter, rID matrix.RoomID, uID matrix.UserID, mods ...MarkupMod) {
	opts := mkopts(mods)

	name, _, _ := uID.Parse()

	if rID != "" {
		n, err := c.MemberName(rID, uID)
		if err == nil {
			name = n.Name
		}
	}

	if opts.at {
		name = "@" + name
	}

	start := iter.Offset()

	color := opts.hasher.Hash(string(uID))

	buf := iter.Buffer()
	buf.Insert(iter, name, len(name))

	startIter := buf.IterAtOffset(start)

	tags := buf.TagTable()

	tag := tags.Lookup("_mauthor_" + string(uID))
	if tag == nil {
		attrs := markuputil.TextTag{
			"foreground": RGBHex(color),
		}
		tag = attrs.Tag("_mauthor_" + string(uID))
		tags.Add(tag)
	}

	buf.ApplyTag(tag, &startIter, iter)
}
