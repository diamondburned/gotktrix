package auth

import (
	"context"
	"log"
	"math"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/auth/secret"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/gotk3/gotk3/glib"
)

const avatarSize = 24

var accountEntryCSS = cssutil.Applier("auth-account-entry", `
	.auth-account-entry {
		padding: 6px 4px;
		min-height: 32px; /* (4px * 2) + 24px */
	}
`)

func newAddEntry() *gtk.ListBoxRow {
	icon := gtk.NewImageFromIconName("list-add-symbolic")
	icon.SetIconSize(gtk.IconSizeNormal)

	addBox := gtk.NewBox(gtk.OrientationHorizontal, 5)
	addBox.Append(icon)
	addBox.Append(gtk.NewLabel("Add an account"))
	addBox.SetHAlign(gtk.AlignCenter)

	row := gtk.NewListBoxRow()
	row.SetChild(addBox)
	accountEntryCSS(row)
	return row
}

var usernameAttrs = markuputil.Attrs(
	pango.NewAttrWeight(pango.WeightBold),
)

var serverAttrs = markuputil.Attrs(
	pango.NewAttrScale(0.8),
	pango.NewAttrWeight(pango.WeightBook),
	pango.NewAttrForegroundAlpha(uint16(math.Round(0.75*65535))),
)

var avatarCSS = cssutil.Applier("auth-avatar", `
	.auth-avatar {
		padding: 4px;
	}
`)

func newAccountEntry(account *account) *gtk.ListBoxRow {
	icon := adw.NewAvatar(avatarSize, account.Username, true)
	avatarCSS(&icon.Widget)
	imgutil.AsyncGET(context.Background(), account.AvatarURL, icon.SetCustomImage)

	name := gtk.NewLabel(account.Username)
	name.SetEllipsize(pango.EllipsizeMiddle)
	name.SetHExpand(true)
	name.SetAttributes(usernameAttrs)

	server := gtk.NewLabel(account.Server)
	server.SetEllipsize(pango.EllipsizeMiddle)
	server.SetHExpand(true)
	server.SetAttributes(serverAttrs)

	grid := gtk.NewGrid()
	grid.Attach(&icon.Widget, 0, 0, 1, 2)
	grid.Attach(name, 1, 0, 1, 1)
	grid.Attach(server, 1, 1, 1, 1)

	row := gtk.NewListBoxRow()
	row.SetChild(grid)
	accountEntryCSS(row)
	return row
}

var accountChooserCSS = cssutil.Applier("account-chooser-step", `
	.account-chooser-step {
		padding: 16px 6px;
	}
`)

func accountChooserStep(a *Assistant) *assistant.Step {
	accountList := gtk.NewListBox()
	accountList.SetSizeRequest(150, -1)
	accountList.Append(newAddEntry())
	accountList.SetSelectionMode(gtk.SelectionBrowse)
	accountList.SetActivateOnSingleClick(true)

	accountList.Connect("row-activated", func(accountList *gtk.ListBox, row *gtk.ListBoxRow) {
		switch ix := row.Index(); {
		case ix == len(a.accounts):
			a.signinPage()
		case ix < len(a.accounts):
			a.chooseAccount(a.accounts[ix])
		default:
			log.Println("unknown index", ix, "chosen")
		}
	})

	loadingSpin := gtk.NewSpinner()
	loadingSpin.SetSizeRequest(16, 16)
	loadingSpin.Start()

	loadingBox := gtk.NewBox(gtk.OrientationVertical, 2)
	loadingBox.SetHAlign(gtk.AlignCenter)
	loadingBox.Append(loadingSpin)

	loadingLabel := gtk.NewLabel("Loading accounts from keyring...")
	loadingLabel.SetWrap(true)
	loadingLabel.SetWrapMode(pango.WrapWordChar)
	loadingBox.Append(loadingLabel)

	// Asynchronously load the accounts.
	go func() {
		accounts, err := loadAccounts(secret.KeyringDriver(keyringAppID))

		glib.IdleAdd(func() {
			if err != nil {
				loadingLabel.SetMarkup(markuputil.Error(err.Error()))
				return
			}

			// Add the accounts into our list in the same order.
			a.accounts = append(a.accounts, accounts...)

			// Use list prepend, so iterate backwards.
			for i := len(accounts) - 1; i >= 0; i-- {
				accountList.Prepend(newAccountEntry(&accounts[i]))
			}
		})
	}()

	box := gtk.NewBox(gtk.OrientationVertical, 14)
	box.Append(accountList)
	box.Append(loadingBox)
	accountChooserCSS(box)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)

	step := assistant.NewStep("Choose an Account", "")
	content := step.ContentArea()
	content.Append(box)

	return step
}
