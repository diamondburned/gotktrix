package mcontent

import (
	"context"

	"github.com/diamondburned/chatkit/components/embed"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/event"
)

type videoContent struct {
	*embed.Embed
	thumbnailURL string
}

var videoCSS = cssutil.Applier("mcontent-video", `
	.mcontent-video {
		padding: 0;
		margin:  0;
		margin-top: 6px;
	}
`)

func newVideoContent(ctx context.Context, msg *event.RoomMessageEvent) contentPart {
	client := gotktrix.FromContext(ctx).Offline()

	videoURL, err := client.MessageMediaURL(msg)
	if err != nil {
		return newUnknownContent(ctx, msg)
	}

	w := maxWidth
	h := maxHeight

	videoInfo, _ := msg.VideoInfo()
	if videoInfo.Width > 0 && videoInfo.Height > 0 {
		w, h = gotktrix.MaxSize(videoInfo.Width, videoInfo.Height, w, h)
	}

	var thumbnailURL string
	if videoInfo.ThumbnailURL != "" {
		thumbnailURL, _ = client.ScaledThumbnail(videoInfo.ThumbnailURL, w, h, gtkutil.ScaleFactor())
	}

	opts := embed.Opts{
		Type:     embed.EmbedTypeVideo,
		Provider: imgutil.HTTPProvider,
	}

	if thumbnailURL == "" {
		opts.Provider = imgutil.FFmpegProvider
		thumbnailURL = videoURL
	}

	embed := embed.New(ctx, w, h, opts)
	embed.SetName(msg.Body)
	embed.SetOpenURL(func() {
		embed.SetFromURL(videoURL)
		embed.ActivateDefault()
	})

	if videoInfo.Width > 0 && videoInfo.Height > 0 {
		renderBlurhash(msg.AdditionalInfo, w, h, func(p *gdkpixbuf.Pixbuf) {
			embed.SetFromURL("")
			embed.Thumbnail.SetPixbuf(p)
		})
	}

	videoCSS(embed)
	return videoContent{
		Embed:        embed,
		thumbnailURL: thumbnailURL,
	}
}

func (c videoContent) LoadMore() {
	c.Embed.SetFromURL(c.thumbnailURL)
}

func (c videoContent) content() {}
