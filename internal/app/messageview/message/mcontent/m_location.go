package mcontent

import (
	"context"
	"fmt"
	"html"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotrix/event"
)

type locationContent struct {
	*gtk.Box
	image *imageContent
	sub   struct {
		*gtk.Box
		icon *gtk.Image
		loc  *gtk.Label
	}
}

func newLocationContent(ctx context.Context, msg *event.RoomMessageEvent) *locationContent {
	c := locationContent{}
	c.sub.icon = gtk.NewImageFromIconName("mark-location-symbolic")

	c.sub.loc = gtk.NewLabel("")
	c.sub.loc.SetWrap(true)
	c.sub.loc.SetWrapMode(pango.WrapWordChar)
	c.sub.loc.SetXAlign(0)
	c.sub.loc.SetHExpand(true)

	lat, long, _, err := msg.GeoURI.Parse()
	if err == nil {
		c.sub.loc.SetMarkup(locale.Sprintf(ctx,
			`%s (<a href="%s">Google Maps</a> or <a href="%s">OpenStreetMap</a>)`,
			html.EscapeString(msg.Body),
			html.EscapeString(googleMapsURL(lat, long)),
			html.EscapeString(openStreetMapURL(lat, long)),
		))
	} else {
		c.sub.loc.SetText(msg.Body)
	}

	c.sub.Box = gtk.NewBox(gtk.OrientationHorizontal, 4)
	c.sub.Append(c.sub.icon)
	c.sub.Append(c.sub.loc)

	c.Box = gtk.NewBox(gtk.OrientationVertical, 2)
	c.Box.Append(c.sub)

	// Check if we have a thumbnail.
	if msg.AdditionalInfo != nil {
		c.image = newImageContent(ctx, msg)
		c.Box.Append(c.image)
	}

	return &c
}

func googleMapsURL(lat, long float64) string {
	return fmt.Sprintf("https://maps.google.com?q=%f,%f", lat, long)
}

func openStreetMapURL(lat, long float64) string {
	return fmt.Sprintf("https://www.openstreetmap.org/#map=14/%f/%f", lat, long)
}

func (c *locationContent) LoadMore() {
	if c.image != nil {
		c.image.LoadMore()
	}
}

func (c *locationContent) content() {}
