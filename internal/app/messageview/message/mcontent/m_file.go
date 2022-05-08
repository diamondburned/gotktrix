package mcontent

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"strings"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/components/filepick"
	"github.com/diamondburned/gotktrix/internal/components/progress"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/event"
	"github.com/dustin/go-humanize"
)

// linkContent is a lighter version of fileContent.
type linkContent struct {
	*gtk.Label
}

func (c *linkContent) content() {}

type fileContent struct {
	*gtk.Box
	ctx context.Context

	info *fileInfo
	brev *gtk.Revealer

	url  string
	name string
	size int
}

type fileInfo struct {
	*gtk.Box
	icon  *gtk.Image
	right struct {
		*gtk.Box
		name *gtk.Label
		size *gtk.Label
	}
	action *gtk.Button
	click  glib.SignalHandle
}

var fileCSS = cssutil.Applier("mcontent-file", `
	.mcontent-file {
		padding: 4px 6px;
	}
	.mcontent-file-info > image {
		padding-right: 6px;
	}
	.mcontent-file-size {
		font-size: 0.85rem;
		margin-top: -2px;
	}
`)

func newFileContent(ctx context.Context, msg *event.RoomMessageEvent) contentPart {
	client := gotktrix.FromContext(ctx)
	url, _ := client.MediaDownloadURL(msg.URL, true, "")

	info, err := msg.FileInfo()
	if err != nil {
		// File has no info for some reason. Just render the URL as a fallback.
		c := linkContent{}
		c.Label = gtk.NewLabel(fmt.Sprintf(
			`<a href="%s">%s</a>`,
			html.EscapeString(url), html.EscapeString(msg.Body),
		))
		c.Label.SetUseMarkup(true)
		c.Label.SetEllipsize(pango.EllipsizeMiddle)
		c.Label.SetXAlign(0)
		c.AddCSSClass("mcontent-file-fallback")
		return &c
	}

	c := fileContent{
		ctx:  ctx,
		url:  url,
		name: msg.Body,
		size: info.Size,
	}
	c.info = newFileInfo(info, c.name, c.url)
	c.info.setAction(fileDownload(c.download))

	c.brev = gtk.NewRevealer()
	c.brev.AddCSSClass("mcontent-file-progress-revealer")
	c.brev.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	c.brev.SetRevealChild(false)

	c.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	c.AddCSSClass("frame")
	c.SetHAlign(gtk.AlignStart)
	c.Append(c.info)
	c.Append(c.brev)
	fileCSS(c.Box)

	return &c
}

func (c *fileContent) download() {
	chooser := filepick.New(
		c.ctx, "Download File",
		gtk.FileChooserActionSave,
		locale.S(c.ctx, "Download"),
		locale.S(c.ctx, "Cancel"),
	)
	chooser.SetCurrentName(c.name)
	chooser.ConnectAccept(func() {
		if path := chooser.File().Path(); path != "" {
			c.downloadTo(path)
		}
	})
	chooser.Show()
}

func (c *fileContent) downloadTo(path string) {
	bar := progress.NewBar()
	bar.SetMax(int64(c.size))

	bar.SetLabelFunc(func(n, max int64) string {
		return fmt.Sprintf(
			"%s (%.0f%%)",
			humanize.Bytes(uint64(n)), float64(n)/float64(max)*100,
		)
	})

	c.brev.SetChild(bar)
	c.brev.SetRevealChild(true)

	ctx, cancel := context.WithCancel(c.ctx)
	c.info.setAction(fileStopDownload(cancel))

	go func() {
		err := progress.Download(ctx, c.url, path, bar)
		cancel()

		glib.IdleAdd(func() {
			c.info.setAction(fileDownload(c.download))
			// Pretend that cancelling the context is not an error.
			if err == nil || errors.Is(err, context.Canceled) {
				c.brev.SetRevealChild(false)
				bar.Unparent()
			}
		})
	}()
}

func newFileInfo(info event.FileInfo, name, url string) *fileInfo {
	icon := "text-x-generic-symbolic"

	if info.MimeType != "" {
		switch strings.Split(info.MimeType, "/")[0] {
		case "application":
			icon = "package-x-generic-symbolic"
		case "image":
			icon = "image-x-generic-symbolic"
		case "video":
			icon = "video-x-generic-symbolic"
		case "audio":
			icon = "audio-x-generic-symbolic"
		}
	}

	inf := fileInfo{}
	inf.icon = gtk.NewImageFromIconName(icon)
	inf.icon.SetIconSize(gtk.IconSizeLarge)

	inf.right.name = gtk.NewLabel(fmt.Sprintf(
		`<a href="%s">%s</a>`,
		html.EscapeString(url), html.EscapeString(name),
	))
	inf.right.name.SetUseMarkup(true)
	inf.right.name.AddCSSClass("mcontent-file-name")
	inf.right.name.SetEllipsize(pango.EllipsizeMiddle)
	inf.right.name.SetXAlign(0)

	inf.right.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	inf.right.Box.SetHExpand(true)
	inf.right.Box.SetVAlign(gtk.AlignCenter)
	inf.right.Box.Append(inf.right.name)

	if info.Size > 0 {
		inf.right.size = gtk.NewLabel(humanize.Bytes(uint64(info.Size)))
		inf.right.size.AddCSSClass("mcontent-file-size")
		inf.right.size.SetEllipsize(pango.EllipsizeMiddle)
		inf.right.size.SetXAlign(0)
		inf.right.Box.Append(inf.right.size)
	}

	inf.action = gtk.NewButton()
	inf.action.SetVAlign(gtk.AlignCenter)
	inf.action.SetSensitive(false)

	inf.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	inf.AddCSSClass("mcontent-file-info")
	inf.SetSizeRequest(maxWidth, -1)
	inf.Append(inf.icon)
	inf.Append(inf.right)
	inf.Append(inf.action)

	return &inf
}

type fileAction interface{ _action() }

type fileDownload func()

type fileStopDownload context.CancelFunc

func (f fileDownload) _action()     {}
func (f fileStopDownload) _action() {}

func (inf *fileInfo) setAction(action fileAction) {
	if inf.click > 0 {
		inf.action.HandlerDisconnect(inf.click)
		inf.click = 0
	}

	inf.action.SetSensitive(true)

	switch action := action.(type) {
	case fileDownload:
		inf.action.SetTooltipText("Download")
		inf.action.SetIconName("folder-download-symbolic")
		inf.click = inf.action.ConnectClicked(action)
	case fileStopDownload:
		inf.action.SetTooltipText("Stop")
		inf.action.SetIconName("process-stop-symbolic")
		inf.click = inf.action.ConnectClicked(func() {
			action()
			inf.action.SetSensitive(false)
		})
	default:
		log.Panicf("unknown action %T", action)
	}
}

func (c *fileContent) content() {}
