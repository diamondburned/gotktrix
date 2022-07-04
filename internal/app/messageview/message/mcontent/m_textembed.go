package mcontent

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"path"
	"strings"

	"github.com/diamondburned/chatkit/components/embed"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/api"
)

var titleAttrs = textutil.Attrs(
	pango.NewAttrWeight(pango.WeightBold),
)

var descAttrs = textutil.Attrs(
	pango.NewAttrWeight(pango.WeightSemilight),
	pango.NewAttrScale(0.85),
)

const (
	embedImageWidth  = 80
	embedImageHeight = 80
)

var embedCSS = cssutil.Applier("mcontent-embed", `
	.mcontent-embed {
		border-left: 3px solid @theme_fg_color;
		padding: 0;
	}
	.mcontent-embed-body {
		margin: 3px 6px;
	}
	.mcontent-embed label {
		margin-right: 3px;
	}
	.mcontent-embeds > *:last-child {
		margin-bottom: 6px;
	}
`)

var descReplacer = strings.NewReplacer("\n", "  ")

var enableEmbeds = prefs.NewBool(true, prefs.PropMeta{
	Name:    "Load Link Embeds",
	Section: "Text",
	Description: "If enabled, query the Matrix server for information about " +
		"links in messages and show them as embeds.",
})

func loadEmbeds(ctx context.Context, box *gtk.Box, urls []string) {
	if !enableEmbeds.Value() {
		return
	}

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
			if err != nil {
				continue
			}

			// Prefer the canonical URL.
			if m.URL == "" {
				m.URL = url
			}

			if m.Title != "" || m.Description != "" {
				addTextEmbed(ctx, box, m)
				continue
			}

			if m.Image != "" {
				// At least we have an image. Render the image fully.
				addImageEmbed(ctx, box, m)
				continue
			}
		}
	}()
}

func addTextEmbed(ctx context.Context, box *gtk.Box, m *api.URLMetadata) {
	m.Description = descReplacer.Replace(m.Description)

	client := gotktrix.FromContext(ctx)
	imageURL, _ := client.ScaledThumbnail(m.Image,
		embedImageWidth, embedImageHeight, gtkutil.ScaleFactor())

	glib.IdleAdd(func() {
		b := gtk.NewBox(gtk.OrientationVertical, 0)
		b.SetHExpand(true)
		b.AddCSSClass("mcontent-embed-body")

		outer := gtk.NewBox(gtk.OrientationHorizontal, 0)
		outer.SetHExpand(true)
		outer.AddCSSClass("mcontent-embed-imagebox")
		outer.Append(b)
		embedCSS(outer)

		if m.Title != "" {
			title := gtk.NewLabel("")
			title.SetMarkup(fmt.Sprintf(
				`<a href="%s">%s</a>`,
				html.EscapeString(m.URL), html.EscapeString(m.Title),
			))
			title.SetXAlign(0)
			title.SetYAlign(0)
			title.SetSingleLineMode(true)
			title.SetEllipsize(pango.EllipsizeEnd)
			title.SetAttributes(titleAttrs)
			title.AddCSSClass("mcontent-embed-title")
			b.Append(title)
		}

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
			img := embed.New(ctx, embedImageWidth, embedImageHeight, embed.Opts{
				Type:     embed.EmbedTypeImage,
				Provider: imgutil.HTTPProvider,
				CanHide:  true,
			})
			img.AddCSSClass("mcontent-embed-thumbnail")
			img.SetHAlign(gtk.AlignEnd)
			img.SetName(path.Base(imageURL))
			img.SetFromURL(imageURL)
			img.SetOpenURL(func() {
				url, _ := gotktrix.FromContext(ctx).MediaDownloadURL(m.Image, true, "")
				app.OpenURI(ctx, url)
			})
			outer.Append(img)
		}

		box.Append(outer)
		box.QueueResize()
	})
}

func addImageEmbed(ctx context.Context, box *gtk.Box, m *api.URLMetadata) {
	// Try and parse the name.
	var name string
	if u, err := url.Parse(m.URL); err == nil {
		name = path.Base(u.Path)
	}

	client := gotktrix.FromContext(ctx)
	imageURL, _ := client.ScaledThumbnail(m.Image, maxWidth, maxHeight, gtkutil.ScaleFactor())

	glib.IdleAdd(func() {
		embed := embed.New(ctx, maxWidth, maxHeight, embed.Opts{
			Type:     embed.EmbedTypeImage,
			Provider: imgutil.HTTPProvider,
		})
		embed.AddCSSClass("mcontent-image-embed")
		embed.SetName(name)
		embed.SetFromURL(imageURL)
		embed.SetOpenURL(func() {
			app.OpenURI(ctx, m.URL)
		})
		box.Append(embed)
		box.QueueResize()
	})
}
