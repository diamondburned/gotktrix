package markuputil

import (
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"html"
	"log"
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

// TextTagsMap describes a map of tag names to its attributes. It is used to
// declaratively construct a TextTagTable using NewTextTags.
type TextTagsMap map[string]TextTag

// TextTag describes a map of attribute/property name to its value for a
// TextTag. Attributes that need a -set suffix will be set to true
// automatically.
type TextTag map[string]interface{}

// TextTagTableFactory creates a function that allocates a new TextTagTable when
// called. The tag tables all share the same allocated tags.
func TextTagTableFactory(m TextTagsMap) func() *gtk.TextTagTable {
	return func() *gtk.TextTagTable {
		table := gtk.NewTextTagTable()

		for name, attrs := range m {
			tag := gtk.NewTextTag(name)
			for k, v := range attrs {
				tag.SetObjectProperty(k, v)
			}

			if !table.Add(tag) {
				log.Panicf("BUG: tag %q not added", name)
			}
		}

		return table
	}
}

// Tag creates a new text tag from the attributes.
func (t TextTag) Tag(name string) *gtk.TextTag {
	tag := gtk.NewTextTag(name)

	for k, v := range t {
		// Edge case.
		if v, ok := v.(string); ok && v == "" {
			continue
		}
		tag.SetObjectProperty(k, v)
	}

	return tag
}

// Hash returns a 24-byte string of the text tag hashed.
func (t TextTag) Hash() string {
	hash := fnv.New128a()

	for k, v := range t {
		hash.Write([]byte(k))
		hash.Write([]byte(":"))
		fmt.Fprintln(hash, v)
	}

	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

// HashTag creates a tag inside the text tag table using the hash of the text
// tag attributes as the name. If the same tag has already been created, then it
// is returned.
func HashTag(table *gtk.TextTagTable, attrs TextTag) *gtk.TextTag {
	hash := "custom." + attrs.Hash()

	if t := table.Lookup(hash); t != nil {
		return t
	}

	tag := attrs.Tag(hash)

	if !table.Add(tag) {
		log.Panicf("text tag hash collision %q", hash)
	}

	return tag
}
