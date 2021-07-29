package syncbox

import (
	"context"
	"log"
	"math"

	"github.com/chanbakjsd/gotrix/api"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/auth"
	"github.com/diamondburned/gotktrix/internal/components/errpopup"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/gotk3/gotk3/glib"
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
	dialog  *gtk.Window
	spinner *gtk.Spinner
	label   *gtk.Label
}

var serverAttrs = markuputil.Attrs(
	pango.NewAttrScale(0.8),
	pango.NewAttrWeight(pango.WeightBook),
	pango.NewAttrForegroundAlpha(uint16(math.Round(0.75*65535))),
)

func newAccountGrid(account *auth.Account) gtk.Widgetter {
	avatar := adw.NewAvatar(avatarSize, account.Username, true)
	imgutil.AsyncGET(context.Background(), account.AvatarURL, avatar.SetCustomImage)

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

// Open shows a popup while opening the client in the background. Once the
// client has successfully synchronized, the popup will close automatically.
// Note that Open will block until the synchronization is done, so it should
// only be called in a goroutine.
func Open(parent *gtk.ApplicationWindow, account *auth.Account, client *gotktrix.Client) {
	syncCh := make(chan *api.SyncResponse, 1)
	var popup *Popup

	glib.IdleAdd(func() {
		popup = Show(&parent.Window, account)
		app := parent.Application()

		go func() {
			client.State.WaitForNextSync(syncCh)

			if err := client.Open(); err != nil {
				errpopup.ShowFatal(&parent.Window, []error{err})
				return
			}

			glib.IdleAddPriority(glib.PRIORITY_HIGH, func() {
				app.Connect("shutdown", func() {
					log.Println("shutting down Matrix...")

					if err := client.Close(); err != nil {
						log.Println("failed to close loop:", err)
					}

					log.Println("Matrix event loop shut down.")
				})
			})
		}()
	})

	// This channel will only unblock once Open() is done syncing, which means
	// popup would've already been set.
	<-syncCh
	glib.IdleAdd(func() { popup.Close() })
}

// Show shows a popup.
func Show(parent *gtk.Window, account *auth.Account) *Popup {
	spinner := gtk.NewSpinner()
	spinner.SetSizeRequest(18, 18)

	loadLabel := gtk.NewLabel("Syncing...")
	loadLabel.SetWrap(true)
	loadLabel.SetWrapMode(pango.WrapWordChar)

	spinBox := gtk.NewBox(gtk.OrientationVertical, 0)
	spinBox.Append(spinner)
	spinBox.Append(loadLabel)

	content := gtk.NewBox(gtk.OrientationVertical, 6)
	content.Append(newAccountGrid(account))
	content.Append(spinBox)
	popupCSS(content)

	handle := gtk.NewWindowHandle()
	handle.SetChild(content)

	window := gtk.NewWindow()
	window.SetTransientFor(parent)
	window.SetModal(true)
	window.SetDefaultSize(250, 100)
	window.SetChild(handle)
	window.SetTitle("Syncing")
	window.SetTitlebar(gtk.NewBox(gtk.OrientationHorizontal, 0)) // no titlebar
	window.Show()

	spinner.Start()

	return &Popup{
		dialog:  window,
		spinner: spinner,
		label:   loadLabel,
	}
}

// Close closes the sync popup.
func (p *Popup) Close() {
	p.spinner.Stop()
	p.dialog.Close()
}

// SetLabel sets the popup's label. The default label is "Syncing".
func (p *Popup) SetLabel(text string) {
	p.label.SetLabel(text)
}
