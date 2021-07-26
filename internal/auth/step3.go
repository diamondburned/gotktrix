package auth

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
)

type loginMethod uint8

const (
	loginPassword loginMethod = iota
	loginToken
)

func methodToggler(a *Assistant, method loginMethod) func() {
	return func() { a.chooseLoginMethod(method) }
}

func chooseLoginStep(a *Assistant) *assistant.Step {
	passwordLogin := gtk.NewButton()
	passwordLogin.SetChild(bigSmallTitleBox(
		"Username/Email",
		"Log in using your username (or email) and password.",
	))
	passwordLogin.Connect("clicked", methodToggler(a, loginPassword))

	tokenLogin := gtk.NewButton()
	tokenLogin.SetChild(bigSmallTitleBox(
		"Token",
		"Log in using your session token",
	))
	tokenLogin.Connect("clicked", methodToggler(a, loginToken))

	step := assistant.NewStep("Log in with", "")
	// step.CanBack = true

	content := step.ContentArea()
	content.SetOrientation(gtk.OrientationVertical)
	content.SetSpacing(6)
	content.Append(passwordLogin)
	content.Append(tokenLogin)

	return step
}

func bigSmallTitleBox(big, small string) gtk.Widgetter {
	bigLabel := gtk.NewLabel(big)
	bigLabel.SetWrap(true)
	bigLabel.SetWrapMode(pango.WrapWordChar)
	bigLabel.SetXAlign(0)
	bigLabel.SetAttributes(markuputil.Attrs(
		pango.NewAttrScale(1.125),
		pango.NewAttrWeight(pango.WeightNormal),
	))

	smallLabel := gtk.NewLabel(small)
	smallLabel.SetWrap(true)
	smallLabel.SetWrapMode(pango.WrapWordChar)
	smallLabel.SetXAlign(0)
	smallLabel.SetAttributes(markuputil.Attrs(
		pango.NewAttrScale(0.9),
		pango.NewAttrWeight(pango.WeightBook),
		pango.NewAttrForegroundAlpha(65535*85/100), // 85% alpha
	))

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(bigLabel)
	box.Append(smallLabel)

	return box
}
