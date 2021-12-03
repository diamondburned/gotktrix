package mcontent

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
)

var titleAttrs = markuputil.Attrs(
	pango.NewAttrWeight(pango.WeightBold),
)

var descAttrs = markuputil.Attrs(
	pango.NewAttrWeight(pango.WeightSemilight),
	pango.NewAttrScale(0.85),
)

const (
	embedImageWidth  = 100
	embedImageHeight = 100
)

var embedCSS = cssutil.Applier("mcontent-embed", `
	.mcontent-embed {
		border-left: 3px solid @theme_fg_color;
		margin-top:  6px;
		padding: 6px 10px;
	}
	.mcontent-embed label {
		margin-right: 6px;
	}
`)

var descReplacer = strings.NewReplacer("\n", "  ")

func loadEmbeds(ctx context.Context, box *gtk.Box, urls []string) {
	// children := make([]gtk.Widgetter, len(urls))

	go func() {
		client := gotktrix.FromContext(ctx)

		// Workaround to keep track of inserted URLs. The actual problem is that
		// edited messages have duplicated URLs for some reason, but this will
		// solve it anyway.
		knownURLs := make(map[string]bool, len(urls))

		for _, url := range urls {
			if knownURLs[url] {
				continue
			} else {
				knownURLs[url] = true
			}

			m, err := client.PreviewURL(url, 0)
			if err != nil || m.Title == "" {
				continue
			}

			if m.URL != "" {
				// Prefer the canonical URL.
				url = m.URL
			}

			m.Description = descReplacer.Replace(m.Description)

			imageURL, _ := client.ScaledThumbnail(m.Image,
				embedImageWidth, embedImageHeight, gtkutil.ScaleFactor())

			glib.IdleAdd(func() {
				var outer gtk.Widgetter

				b := gtk.NewBox(gtk.OrientationVertical, 0)
				b.SetHExpand(true)
				b.AddCSSClass("mcontent-embed-body")

				title := gtk.NewLabel("")
				title.SetMarkup(fmt.Sprintf(
					`<a href="%s">%s</a>`,
					html.EscapeString(url), html.EscapeString(m.Title),
				))
				title.SetXAlign(0)
				title.SetYAlign(0)
				title.SetSingleLineMode(true)
				title.SetEllipsize(pango.EllipsizeEnd)
				title.SetAttributes(titleAttrs)
				title.AddCSSClass("mcontent-embed-title")
				b.Append(title)

				if m.Description != "" {
					desc := gtk.NewLabel(m.Description)
					desc.SetXAlign(0)
					desc.SetYAlign(0)
					desc.SetLines(4)
					desc.SetEllipsize(pango.EllipsizeEnd)
					desc.SetWrapMode(pango.WrapWordChar)
					desc.SetOverflow(gtk.OverflowHidden)
					desc.SetAttributes(descAttrs)
					desc.AddCSSClass("mcontent-embed-description")
					b.Append(desc)
				}

				if imageURL != "" {
					img := gtk.NewImage()
					img.SetHAlign(gtk.AlignEnd)
					imgutil.AsyncGET(
						ctx, imageURL, img.SetFromPaintable,
						imgutil.WithSizeOverrider(img, embedImageWidth, embedImageHeight),
					)

					imgBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
					imgBox.SetHExpand(true)
					imgBox.AddCSSClass("mcontent-embed-imagebox")
					imgBox.Append(b)
					imgBox.Append(img)
					embedCSS(imgBox)
					outer = imgBox
				} else {
					embedCSS(b)
					outer = b
				}

				box.Append(outer)
			})
		}
	}()
}
