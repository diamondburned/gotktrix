package markuputil

import (
	"fmt"
	"html"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

// Attrs is a way to declaratively create a pango.AttrList.
func Attrs(attrs ...*pango.Attribute) *pango.AttrList {
	list := pango.NewAttrList()
	for _, attr := range attrs {
		list.Insert(attr)
	}
	return list
}

// Error formats the given message red.
func Error(msg string) string {
	msg = strings.TrimPrefix(msg, "error ")
	return fmt.Sprintf(
		`<span color="#FF0033"><b>Error:</b> %s</span>`,
		html.EscapeString(msg),
	)
}

// ErrorLabel makes a new label with the class `.error'.
func ErrorLabel(markup string) *gtk.Label {
	errLabel := gtk.NewLabel(markup)
	errLabel.SetUseMarkup(true)
	errLabel.SetSelectable(true)
	errLabel.SetWrap(true)
	errLabel.SetWrapMode(pango.WrapWordChar)
	errLabel.SetCSSClasses([]string{"error"})
	errLabel.SetAttributes(Attrs(pango.NewAttrInsertHyphens(false)))
	return errLabel
}
