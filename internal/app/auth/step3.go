package auth

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/components/assistant"
	"github.com/diamondburned/gotrix/matrix"
)

func methodToggler(a *Assistant, method matrix.LoginMethod) func() {
	return func() { a.chooseLoginMethod(method) }
}

var supportedLoginMethods = map[matrix.LoginMethod]bool{
	matrix.LoginPassword: true,
	matrix.LoginToken:    true,
	matrix.LoginSSO:      true,
}

func chooseLoginStep(a *Assistant, methods []matrix.LoginMethod) *assistant.Step {
	step := assistant.NewStep("Log in with", "")
	step.CanBack = true

	content := step.ContentArea()
	content.SetOrientation(gtk.OrientationVertical)
	content.SetSpacing(6)

	if hasLoginMethod(methods, matrix.LoginPassword) {
		content.Append(loginMethodButton(a, matrix.LoginPassword,
			"Username/Email",
			"Log in using your username (or email) and password.",
		))
	}

	if hasLoginMethod(methods, matrix.LoginToken) {
		content.Append(loginMethodButton(a, matrix.LoginToken,
			"Token",
			"Log in using your session token",
		))
	}

	if hasLoginMethod(methods, matrix.LoginSSO) {
		content.Append(loginMethodButton(a, matrix.LoginSSO,
			"Single Sign-on (SSO)",
			"Log in using a third-party service",
		))
	}

	return step
}

func hasLoginMethod(methods []matrix.LoginMethod, method matrix.LoginMethod) bool {
	for _, m := range methods {
		if m == method {
			return true
		}
	}
	return false
}

func loginMethodButton(a *Assistant, method matrix.LoginMethod, big, small string) *gtk.Button {
	button := gtk.NewButton()
	button.SetChild(bigSmallTitleBox(big, small))
	button.ConnectClicked(methodToggler(a, method))
	return button
}

func bigSmallTitleBox(big, small string) gtk.Widgetter {
	bigLabel := gtk.NewLabel(big)
	bigLabel.SetWrap(true)
	bigLabel.SetWrapMode(pango.WrapWordChar)
	bigLabel.SetXAlign(0)
	bigLabel.SetAttributes(textutil.Attrs(
		pango.NewAttrScale(1.125),
		pango.NewAttrWeight(pango.WeightNormal),
	))

	smallLabel := gtk.NewLabel(small)
	smallLabel.SetWrap(true)
	smallLabel.SetWrapMode(pango.WrapWordChar)
	smallLabel.SetXAlign(0)
	smallLabel.SetAttributes(textutil.Attrs(
		pango.NewAttrScale(0.9),
		pango.NewAttrWeight(pango.WeightBook),
		pango.NewAttrForegroundAlpha(65535*85/100), // 85% alpha
	))

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(bigLabel)
	box.Append(smallLabel)

	return box
}
