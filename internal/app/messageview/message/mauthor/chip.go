package mauthor

import (
	"context"
	"fmt"
	"log"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
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

	ctx  context.Context
	room matrix.RoomID
	user matrix.UserID
}

var chipCSS = cssutil.Applier("mauthor-chip", `
	.mauthor-chip {
		border-radius: 9999px 9999px;
		margin-bottom: -0.3em;
	}
	.mauthor-chip-colored {
		background-color: transparent; /* override custom CSS */
	}
`)

// NewChip creates a new Chip widget.
func NewChip(ctx context.Context, room matrix.RoomID, user matrix.UserID) *Chip {
	c := Chip{
		ctx:  ctx,
		room: room,
		user: user,
	}

	c.name = gtk.NewLabel("")
	c.name.AddCSSClass("mauthor-chip-colored")

	c.avatar = adaptive.NewAvatar(20)
	c.avatar.ConnectLabel(c.name)

	c.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	c.Box.SetOverflow(gtk.OverflowHidden)
	c.Box.Append(c.avatar)
	c.Box.Append(c.name)
	chipCSS(c)

	gtkutil.OnFirstMap(c, func() {
		color := UserColor(user, WithWidgetColor(c))
		addCustomCSS(customChipCSS(color), c.name, c.Box)
	})

	c.Invalidate()

	return &c
}

const chipTagName = "__mauthor_chip"

// InsertText inserts the chip into the given TextView at the given TextIter.
// The inserted anchor is returned.
func (c *Chip) InsertText(text *gtk.TextView, iter *gtk.TextIter) *gtk.TextChildAnchor {
	buffer := text.Buffer()

	startOffset := iter.Offset()

	anchor := buffer.CreateChildAnchor(iter)
	text.AddChildAtAnchor(c, anchor)

	start := buffer.IterAtOffset(startOffset)

	tags := buffer.TagTable()

	tag := tags.Lookup(chipTagName)
	if tag == nil {
		tag = gtk.NewTextTag(chipTagName)
		tag.SetObjectProperty("rise", -2*pango.SCALE)
		tag.SetObjectProperty("rise-set", true)
		tags.Add(tag)
	}

	buffer.ApplyTag(tag, start, iter)
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
	c.setName(Name(client, c.room, c.user))

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

	avatarURL, _ := client.SquareThumbnail(*mxc, 22, gtkutil.ScaleFactor())
	imgutil.AsyncGET(
		c.ctx, avatarURL, c.avatar.SetFromPaintable,
		imgutil.WithErrorFn(func(err error) {
			log.Print("error getting avatar ", mxc, ": ", err)
		}),
	)
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
