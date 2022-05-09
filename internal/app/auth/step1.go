package auth

import (
	"context"
	"log"
	"math"
	"sync"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/secret"
	"github.com/pkg/errors"
)

const avatarSize = 32

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

var usernameAttrs = textutil.Attrs(
// pango.NewAttrWeight(pango.WeightBold),
)

var serverAttrs = textutil.Attrs(
	pango.NewAttrScale(0.8),
	pango.NewAttrWeight(pango.WeightBook),
	pango.NewAttrForegroundAlpha(uint16(math.Round(0.75*65535))),
)

var avatarCSS = cssutil.Applier("auth-avatar", `
	.auth-avatar {
		margin:  4px;
		padding: 0;
	}
`)

func newAccountEntry(ctx context.Context, account *Account) *gtk.ListBoxRow {
	avatar := onlineimage.NewAvatar(ctx, gotktrix.AvatarProvider, avatarSize)
	avatar.SetInitials(account.Username)
	avatar.SetFromURL(account.AvatarURL)
	avatarCSS(avatar)

	name := gtk.NewLabel(account.Username)
	name.SetXAlign(0)
	name.SetEllipsize(pango.EllipsizeMiddle)
	name.SetHExpand(true)
	name.SetAttributes(usernameAttrs)

	server := gtk.NewLabel(account.Server)
	server.SetXAlign(0)
	server.SetEllipsize(pango.EllipsizeMiddle)
	server.SetHExpand(true)
	server.SetAttributes(serverAttrs)

	grid := gtk.NewGrid()
	grid.SetColumnSpacing(2)
	grid.Attach(avatar, 0, 0, 1, 2)
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
	accountList.SetSizeRequest(250, -1)
	accountList.Append(newAddEntry())
	accountList.SetSelectionMode(gtk.SelectionBrowse)
	accountList.SetActivateOnSingleClick(true)

	loadingSpin := gtk.NewSpinner()
	loadingSpin.SetSizeRequest(16, 16)
	loadingSpin.Start()

	tailbox := gtk.NewBox(gtk.OrientationVertical, 2)
	tailbox.SetVExpand(true)
	tailbox.SetVAlign(gtk.AlignEnd)
	tailbox.SetHAlign(gtk.AlignCenter)
	tailbox.Append(loadingSpin)

	// Use a waitgroup to wait for both goroutines to finish its tasks.
	var wg sync.WaitGroup
	wg.Add(2)

	childCount := 2
	minusChild := func() {
		childCount--
		if childCount == 0 {
			loadingSpin.Stop()
			tailbox.Remove(loadingSpin)
		}
	}

	keyringStatus := gtk.NewLabel("Loading accounts from keyring...")
	keyringStatus.SetWrap(true)
	keyringStatus.SetWrapMode(pango.WrapWordChar)
	tailbox.Append(keyringStatus)

	// Asynchronously load the accounts.
	go func() {
		accounts, err := loadAccounts(a.ctx, a.keyring)

		glib.IdleAdd(func() {
			minusChild()
			defer wg.Done()

			if errors.Is(err, secret.ErrUnsupportedPlatform) {
				// Invalidate the keyring now while we can. This will aid
				// visually in step 4.
				a.keyring = nil

				tailbox.Remove(keyringStatus)
				return
			}

			if err != nil {
				keyringStatus.Show()
				keyringStatus.SetMarkup(textutil.ErrorMarkup(err.Error()))
				return
			}

			tailbox.Remove(keyringStatus)
			addAccounts(a, accountList, a.keyring, accounts)
		})
	}()

	go func() {
		if !secret.PathIsEncrypted(a.encryptPath) {
			glib.IdleAdd(func() {
				minusChild()
				wg.Done()
			})
			return
		}

		glib.IdleAdd(func() {
			minusChild()
			defer wg.Done()

			// Make a password prompt and add it.
			button := gtk.NewButtonWithLabel("Decrypt")

			password := gtk.NewEntry()
			password.SetPlaceholderText("Decrypt local accounts")
			password.SetHExpand(true)
			password.SetVisibility(false)
			password.SetInputPurpose(gtk.InputPurposePassword)

			errLabel := makeErrorLabel()
			errLabel.Hide()

			box := gtk.NewBox(gtk.OrientationHorizontal, 2)
			box.Append(password)
			box.Append(button)

			tailbox.Append(errLabel)
			tailbox.Append(box)

			password.ConnectActivate(func() { button.Activate() })
			button.ConnectClicked(func() {
				// Populate the encryption for step 4.
				a.encrypt = secret.EncryptedFileDriver(password.Text(), a.encryptPath)
				// Add to the waitgroup and wait until decryption is done.
				wg.Add(1)
				// Disable button.
				button.SetSensitive(false)

				go func() {
					accounts, err := loadAccounts(a.ctx, a.encrypt)

					glib.IdleAdd(func() {
						defer wg.Done()

						if err != nil {
							errLabel.SetMarkup(textutil.ErrorMarkup(err.Error()))
							errLabel.Show()
							return
						}

						tailbox.Remove(errLabel)
						tailbox.Remove(box)
						addAccounts(a, accountList, a.encrypt, accounts)
					})
				}()
			})
		})
	}()

	errLabel := makeErrorLabel()
	errLabel.Hide()

	onError := func(err error) {
		errLabel.SetMarkup(textutil.ErrorMarkup(err.Error()))
		errLabel.Show()
		a.Continue()
	}

	useExistingAccount := func(row *gtk.ListBoxRow) {
		acc := a.accounts[row.Index()]
		ctx := a.CancellableBusy(a.ctx)

		go func() {
			c, err := gotktrix.New(acc.Server, acc.Token, gotktrix.Opts{
				Client:     a.client.WithContext(ctx),
				ConfigPath: app.FromContext(ctx),
			})
			if err != nil {
				err = errors.Wrap(err, "server error")
				glib.IdleAdd(func() {
					// Disable the account row, since it's not working anyway.
					// Don't delete the account, though.
					row.SetSensitive(false)
					onError(err)
				})
				return
			}

			if newAcc, err := copyAccount(c); err == nil {
				if err := saveAccount(acc.src, newAcc); err != nil {
					log.Println("error updating old account:", err)
				}
			}

			glib.IdleAdd(func() {
				a.finish(c.WithContext(a.ctx), acc.Account)
			})
		}()
	}

	box := gtk.NewBox(gtk.OrientationVertical, 14)
	box.Append(accountList)
	box.Append(errLabel)
	box.Append(tailbox)
	accountChooserCSS(box)

	step := assistant.NewStep("Choose an Account", "")
	step.Done = func(step *assistant.Step) {
		// Mark the assistant dialog as busy so we can wait for the idling jobs,
		// if any.
		a.Busy()

		go func() {
			wg.Wait()

			glib.IdleAdd(func() {
				// Resume the assistant and continue.
				step.Assistant().NextStep()
			})
		}()
	}

	accountList.ConnectRowActivated(func(row *gtk.ListBoxRow) {
		switch ix := row.Index(); {
		case ix == len(a.accounts):
			a.signinPage()
		case ix < len(a.accounts):
			useExistingAccount(row)
		default:
			log.Println("unknown index", ix, "chosen")
		}
	})

	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(box)

	content := step.ContentArea()
	content.SetVAlign(gtk.AlignFill)
	content.Append(scroll)

	return step
}

func addAccounts(a *Assistant, accountList *gtk.ListBox, src secret.Driver, accounts []Account) {
	hasAccount := func(has *Account) bool {
		for _, acc := range a.accounts {
			if acc.UserID == has.UserID {
				return true
			}
		}
		return false
	}

	// Use list prepend, so iterate backwards.
	for i := len(accounts) - 1; i >= 0; i-- {
		account := &accounts[i]

		if hasAccount(account) {
			// Duplicate.
			continue
		}

		a.accounts = append([]assistantAccount{{
			Account: account,
			src:     src,
		}}, a.accounts...)
		accountList.Prepend(newAccountEntry(a.ctx, account))
	}
}
