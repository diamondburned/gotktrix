package about

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
)

// Show shows the about dialog.
func Show(ctx context.Context) *gtk.AboutDialog {
	// TODO: add go.mod parsing for authors maybe

	about := gtk.NewAboutDialog()
	about.SetTransientFor(app.GTKWindowFromContext(ctx))
	about.SetModal(true)
	about.SetProgramName("gotktrix")
	about.SetVersion("git") // TODO version
	about.SetWebsite("https://github.com/diamondburned/gotktrix")
	about.SetWebsiteLabel("Source code")
	about.SetLicenseType(gtk.LicenseAGPL30)
	about.SetAuthors([]string{
		"diamondburned",
		"chanbakjsd (library)",
	})
	about.Show()

	return about
}
