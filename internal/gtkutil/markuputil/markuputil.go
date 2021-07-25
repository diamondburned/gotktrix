package markuputil

import (
	"fmt"
	"html"

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
	return fmt.Sprintf(
		`<span color="red"><b>Error:</b> %s</span>`,
		html.EscapeString(msg),
	)
}
