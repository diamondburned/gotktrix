// Package mauthor handles rendering Matrix room members' names.
package mauthor

import (
	"fmt"
	"html"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/pronouns"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

type markupOpts struct {
	textTag textutil.TextTag
	hasher  ColorHasher
	name    string
	at      bool
	shade   bool
	minimal bool
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

// WithShade renders the markup with a background shade.
func WithShade() MarkupMod {
	return func(opts *markupOpts) {
		opts.shade = true
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
func WithWidgetColor() MarkupMod {
	if textutil.IsDarkTheme() {
		return WithColorHasher(LightColorHasher)
	} else {
		return WithColorHasher(DarkColorHasher)
	}
}

// WithTextTagAttr sets the given attribute into the text tag used for the
// author. It is only useful for Text.
func WithTextTagAttr(attr textutil.TextTag) MarkupMod {
	return func(opts *markupOpts) {
		opts.textTag = attr
	}
}

// WithName overrides the name in the generated author string.
func WithName(name string) MarkupMod {
	return func(opts *markupOpts) {
		opts.name = name
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

// UserColor returns the color in hexadecimal with the # prefix.
func UserColor(uID matrix.UserID, mods ...MarkupMod) string {
	return userColor(uID, mkopts(mods))
}

func userColor(uID matrix.UserID, opts markupOpts) string {
	name, _, err := uID.Parse()
	if err != nil {
		name = string(uID)
	}

	return RGBHex(opts.hasher.Hash(name))
}

// Name returns the raw name string with no markup.
func Name(c *gotktrix.Client, rID matrix.RoomID, uID matrix.UserID, mods ...MarkupMod) string {
	c = c.Offline()
	opts := mkopts(mods)

	name, _, _ := uID.Parse()
	if name == "" {
		name = string(uID)
	}

	if rID != "" {
		n, err := c.MemberName(rID, uID, !opts.minimal)
		if err == nil && n.Name != "" {
			name = n.Name
		}
	}

	if opts.name != "" {
		name = opts.name
	}

	if opts.at && !strings.HasPrefix(name, "@") {
		name = "@" + name
	}

	return name
}

// Markup renders the markup string for the given user inside the given room.
// The markup format follows the Pango markup format.
//
// If the given room ID string is empty, then certain information are skipped.
// If it's non-empty, then the state will be used to fetch additional
// information.
func Markup(c *gotktrix.Client, rID matrix.RoomID, uID matrix.UserID, mods ...MarkupMod) string {
	// TODO: maybe bridge role colors?

	c = c.Offline()
	opts := mkopts(mods)

	name, _, _ := uID.Parse()
	if name == "" {
		name = string(uID)
	}

	var ambiguous bool

	if rID != "" {
		n, err := c.MemberName(rID, uID, !opts.minimal)
		if err == nil && n.Name != "" {
			name = n.Name
			ambiguous = n.Ambiguous
		}
	}

	if opts.name != "" {
		name = opts.name
	}

	if opts.at && !strings.HasPrefix(name, "@") {
		name = "@" + name
	}

	b := strings.Builder{}
	b.Grow(512)
	if opts.shade {
		b.WriteString(fmt.Sprintf(
			`<span color="%s" bgcolor="%[1]s33">%s</span>`,
			userColor(uID, opts), html.EscapeString(name),
		))
	} else {
		b.WriteString(fmt.Sprintf(
			`<span color="%s">%s</span>`,
			userColor(uID, opts), html.EscapeString(name),
		))
	}

	if opts.minimal {
		return b.String()
	}

	if pronoun := pronouns.UserPronouns(c, rID, uID).Pronoun(); pronoun != "" {
		b.WriteByte(' ')
		b.WriteString(fmt.Sprintf(
			`<span fgalpha="90%%" size="small">(%s)</span>`,
			html.EscapeString(string(pronoun)),
		))
	}

	if ev, _ := c.RoomState(rID, event.TypeRoomMember, string(uID)); ev != nil {
		member := m.DiscordMemberFromMatrix(ev.(*event.RoomMemberEvent))
		if member != nil {
			b.WriteByte(' ')
			b.WriteString(fmt.Sprintf(
				`<span fgalpha="75%%" size="small">(%s)</span>`,
				member.Username,
			))
			// We can show the username directly instead of the Matrix ID for
			// Discord users.
			ambiguous = false
		}
	}

	if ambiguous {
		b.WriteByte(' ')
		b.WriteString(fmt.Sprintf(
			`<span fgalpha="75%%" size="small">(%s)</span>`,
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
	if name == "" {
		name = string(uID)
	}

	if rID != "" {
		n, err := c.MemberName(rID, uID, !opts.minimal)
		if err == nil {
			name = n.Name
		}
	}

	if opts.name != "" {
		name = opts.name
	}

	if opts.at && !strings.HasPrefix(name, "@") {
		name = "@" + name
	} else if name == "" {
		return
	}

	start := iter.Offset()

	buf := iter.Buffer()
	buf.Insert(iter, name)

	startIter := buf.IterAtOffset(start)

	tags := buf.TagTable()

	tag := tags.Lookup("_mauthor_" + string(uID))
	if tag == nil {
		color := userColor(uID, opts)
		attrs := textutil.TextTag{
			"foreground": color,
		}
		if opts.shade {
			attrs["background"] = color + "33" // alpha
		}
		if opts.textTag != nil {
			for k, v := range opts.textTag {
				attrs[k] = v
			}
		}
		tag = attrs.Tag("_mauthor_" + string(uID))
		tags.Add(tag)
	}

	buf.ApplyTag(tag, startIter, iter)
}
