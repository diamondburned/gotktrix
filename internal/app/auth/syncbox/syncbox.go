package syncbox

import (
	"context"
	"log"
	"math"

	"github.com/chanbakjsd/gotrix/api"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/auth"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
)

const avatarSize = 36

var popupCSS = cssutil.Applier("syncbox-popup", `
	.syncbox-popup {
		padding: 6px 4px;
	}
	.syncbox-popup > grid {
		margin-left: 2px;
	}
`)

type Popup struct {
	app     *app.Application
	spinner *gtk.Spinner
	label   *gtk.Label
}

var serverAttrs = markuputil.Attrs(
	pango.NewAttrScale(0.8),
	pango.NewAttrWeight(pango.WeightBook),
	pango.NewAttrForegroundAlpha(uint16(math.Round(0.75*65535))),
)

func newAccountGrid(ctx context.Context, account *auth.Account) gtk.Widgetter {
	avatar := adw.NewAvatar(avatarSize, account.Username, true)
	imgutil.AsyncGET(ctx, account.AvatarURL, avatar.SetCustomImage)

	name := gtk.NewLabel(account.Username)
	name.SetXAlign(0)
	name.SetEllipsize(pango.EllipsizeMiddle)
	name.SetHExpand(true)

	server := gtk.NewLabel(account.Server)
	server.SetXAlign(0)
	server.SetEllipsize(pango.EllipsizeMiddle)
	server.SetHExpand(true)
	server.SetAttributes(serverAttrs)

	grid := gtk.NewGrid()
	grid.SetColumnSpacing(6)
	grid.Attach(&avatar.Widget, 0, 0, 1, 2)
	grid.Attach(name, 1, 0, 1, 1)
	grid.Attach(server, 1, 1, 1, 1)

	return grid
}

// OpenThen is similar to Open, except the function does not block, but instead
// will call f in the main event loop once it's done.
func OpenThen(ctx context.Context, acc *auth.Account, f func()) {
	if f == nil {
		panic("given callback must not be nil")
	}

	openThen(ctx, acc, f)
}

// Open shows a popup while opening the client in the background. Once the
// client has successfully synchronized, the popup will close automatically.
// Note that Open will block until the synchronization is done, so it should
// only be called in a goroutine.
func Open(ctx context.Context, acc *auth.Account) *Popup {
	return openThen(ctx, acc, nil)
}

func openThen(ctx context.Context, acc *auth.Account, f func()) *Popup {
	client := gotktrix.FromContext(ctx)
	syncCh := make(chan *api.SyncResponse, 1)

	ctx, cancel := context.WithCancel(ctx)
	client.OnSyncCh(ctx, syncCh)

	var popup *Popup

	glib.IdleAdd(func() {
		popup = Show(ctx, acc)

		go func() {
			if err := client.Open(); err != nil {
				app.Fatal(ctx, err)
				cancel()
				return
			}

			app.FromContext(ctx).Connect("shutdown", func() {
				log.Println("shutting down Matrix...")

				if err := client.Close(); err != nil {
					log.Println("failed to close loop:", err)
				}

				log.Println("Matrix event loop shut down.")
			})

			if f != nil {
				<-syncCh
				cancel()
				glib.IdleAdd(f)
			}
		}()
	})

	if f == nil {
		// This channel will only unblock once Open() is done syncing, which
		// means popup would've already been set.
		<-syncCh
		cancel()
		return popup
	}

	return nil
}

// Show shows a popup.
func Show(ctx context.Context, account *auth.Account) *Popup {
	spinner := gtk.NewSpinner()
	spinner.SetSizeRequest(18, 18)

	loadLabel := gtk.NewLabel("Syncing...")
	loadLabel.SetWrap(true)
	loadLabel.SetWrapMode(pango.WrapWordChar)

	spinBox := gtk.NewBox(gtk.OrientationVertical, 0)
	spinBox.Append(spinner)
	spinBox.Append(loadLabel)

	content := gtk.NewBox(gtk.OrientationVertical, 6)
	content.Append(newAccountGrid(ctx, account))
	content.Append(spinBox)
	content.SetHExpand(true)
	content.SetVExpand(true)
	content.SetHAlign(gtk.AlignCenter)
	content.SetVAlign(gtk.AlignCenter)
	popupCSS(content)

	app := app.FromContext(ctx)
	app.Window().SetChild(content)
	app.SetTitle("Syncing")
	app.NotifyChild(true, func() { spinner.Stop() })

	spinner.Start()

	return &Popup{
		app:     app,
		spinner: spinner,
		label:   loadLabel,
	}
}

// SetLabel sets the popup's label. The default label is "Syncing".
func (p *Popup) SetLabel(text string) {
	p.app.SetTitle(text)
	p.label.SetLabel(text)
}

// QueueSetLabel queues the label setting to be on the main thread.
func (p *Popup) QueueSetLabel(text string) {
	glib.IdleAdd(func() { p.SetLabel(text) })
}
