package mauthor

import (
	"context"
	"fmt"
	"log"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/matrix"
)

// Chip describes a user chip. It is used for display in messages and for
// composing messages in the composer bar.
//
// In Material Design, chips are described as "compact elements tha represen an
// input, attribute, or action." People who have used Google Mail before will
// have seen it when they input an email address into the "To" field and having
// it turn into a small box showing the user information in a friendlier way
// instead of being an email address.
//
// Note that due to how chips are currently implemented, it only works well at
// certain font scale ranges. Once the font scale is beyond ~1.3, flaws will
// start to be very apparent. In that case, the user should use proper graphics
// scaling using Wayland, not using hacks like font scaling.
type Chip struct {
	*gtk.Box
	avatar *adaptive.Avatar
	name   *gtk.Label
	mods   []MarkupMod

	ctx  context.Context
	room matrix.RoomID
	user matrix.UserID
}

var chipCSS = cssutil.Applier("mauthor-chip", `
	.mauthor-chip {
		border-radius: 9999px 9999px;
		margin-bottom: -0.4em;
	}
	.mauthor-chip-unpadded {
		margin-bottom: 0;
	}
	.mauthor-chip-colored {
		background-color: transparent; /* override custom CSS */
		margin: -1px 0;
	}
	/*
     * Workaround for GTK padding an extra line at the bottom of the TextView if
	 * even one widget is inserted for some weird reason.
     */
	.mauthor-haschip {
		margin-bottom: -1em;
	}
`)

// NewChip creates a new Chip widget.
func NewChip(ctx context.Context, room matrix.RoomID, user matrix.UserID, mods ...MarkupMod) *Chip {
	c := Chip{
		ctx:  ctx,
		room: room,
		user: user,
		mods: mods,
	}

	c.name = gtk.NewLabel("")
	c.name.AddCSSClass("mauthor-chip-colored")
	c.name.SetXAlign(0.4) // account for the right round corner

	c.avatar = adaptive.NewAvatar(0)
	c.avatar.ConnectLabel(c.name)

	c.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	c.Box.SetOverflow(gtk.OverflowHidden)
	c.Box.Append(c.avatar)
	c.Box.Append(c.name)
	chipCSS(c)

	gtkutil.OnFirstDrawUntil(c.name, func() bool {
		// Update the avatar size using the Label's height for consistency. From
		// my experiments, the Label's height is 21, so 21 or 22 would've
		// sufficed, but we're doing this just to make sure the chip is only as
		// tall as it needs to be.
		h := c.name.AllocatedHeight()
		if h < 1 {
			return true
		}
		c.avatar.SetSizeRequest(h)
		return false
	})

	gtkutil.OnFirstMap(c, func() {
		// Update the color using CSS.
		color := UserColor(user, WithWidgetColor())
		addCustomCSS(customChipCSS(color), c.name, c.Box)
	})

	c.Invalidate()

	return &c
}

// Unpad removes the negative margin in the Chip.
func (c *Chip) Unpad() {
	c.AddCSSClass("mauthor-chip-unpadded")
}

// InsertText inserts the chip into the given TextView at the given TextIter.
// The inserted anchor is returned.
func (c *Chip) InsertText(text *gtk.TextView, iter *gtk.TextIter) *gtk.TextChildAnchor {
	buffer := text.Buffer()

	anchor := buffer.CreateChildAnchor(iter)
	text.AddChildAtAnchor(c, anchor)

	text.AddCSSClass("mauthor-haschip")
	text.QueueResize()

	return anchor
}

// RoomID returns the room ID that the chip is showing.
func (c *Chip) RoomID() matrix.RoomID { return c.room }

// UserID returns the user ID that the chip is showing.
func (c *Chip) UserID() matrix.UserID { return c.user }

// Name returns the visible name that the chip is showing.
func (c *Chip) Name() string { return c.name.Text() }

// Invalidate invalidates the state of the chip and asks it to update the name
// and avatar. The NewChip constructor will automatically call this method, so
// the user doesn't have to.
func (c *Chip) Invalidate() {
	// This does not query from the API at all. It's probably not very important
	// to do so.
	client := gotktrix.FromContext(c.ctx).Offline()
	c.setName(Name(client, c.room, c.user, c.mods...))

	url, err := client.MemberAvatar(c.room, c.user)
	if err == nil {
		c.setAvatar(client, url)
	}
}

func (c *Chip) setAvatar(client *gotktrix.Client, mxc *matrix.URL) {
	if mxc == nil {
		c.avatar.SetFromPaintable(nil)
		return
	}

	ctx := imgutil.WithOpts(c.ctx,
		imgutil.WithErrorFn(func(err error) {
			log.Print("error getting avatar ", mxc, ": ", err)
		}),
	)

	avatarURL, _ := client.SquareThumbnail(*mxc, 24, gtkutil.ScaleFactor())

	imgutil.AsyncGET(ctx, avatarURL, imgutil.ImageSetter{
		SetFromPaintable: c.avatar.SetFromPaintable,
		SetFromPixbuf:    c.avatar.SetFromPixbuf,
	})
}

const maxChipWidth = 200

func (c *Chip) setName(label string) {
	c.name.SetEllipsize(pango.EllipsizeNone)
	c.name.SetText(label)

	// Properly limit the size of the label.
	layout := c.name.Layout()

	width, _ := layout.PixelSize()
	width += 8 // padding

	if width > maxChipWidth {
		width = maxChipWidth
	}

	c.name.SetSizeRequest(width, -1)
	c.name.SetEllipsize(pango.EllipsizeEnd)
}

// customChipCSSf is the CSS fmt string that's specific to each user. The 0.8 is
// taken from the 0x33 alpha: 0x33/0xFF = 0.2.
const customChipCSSf = `
	box {
		background-color: mix(%[1]s, @theme_bg_color, 0.8);
	}
	label {
		color: %[1]s;
	}
`

func customChipCSS(hex string) gtk.StyleProviderer {
	// There doesn't seem to be a better way than this...
	css := gtk.NewCSSProvider()
	css.LoadFromData(fmt.Sprintf(customChipCSSf, hex))
	return css
}

func addCustomCSS(provider gtk.StyleProviderer, widgets ...gtk.Widgetter) {
	for _, widget := range widgets {
		w := gtk.BaseWidget(widget)
		w.StyleContext().AddProvider(provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	}
}
