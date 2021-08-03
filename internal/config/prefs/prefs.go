// Package prefs provides a publish-subscription API for global settinsg
// management. It exists as a schema-less version of the GSettings API.
package prefs

import (
	"log"
	"sort"
	"strings"
	"unicode"

	"github.com/diamondburned/gotktrix/internal/sortutil"
)

// Slug describes a particular slug format type.
type Slug string

// Slugify turns any string into an ID string.
func Slugify(any string) Slug {
	return Slug(strings.Map(slugify, any))
}

func slugify(r rune) rune {
	if r == '/' {
		return '-'
	}
	if unicode.IsSpace(r) {
		return '-'
	}
	return unicode.ToLower(r)
}

// ID describes a property ID type.
type ID Slug

// SectionID describes a property's section ID type.
type SectionID Slug

// PropMeta describes the metadata of a preference value.
type PropMeta struct {
	Name        string
	Section     string
	Description string
}

// ID returns an ID from the preference name.
func (p *PropMeta) ID() ID {
	id := ID(Slugify(p.Name))
	if p.Section != "" {
		id = ID(p.SectionID()) + "/" + id
	}
	return id
}

// SectionID returns the section ID, or an empty string if there is no section.
func (p *PropMeta) SectionID() SectionID {
	return SectionID(Slugify(p.Section))
}

func (p *PropMeta) validate() {
	if p.Name == "" {
		log.Panicln("missing prop name")
	}
}

// Prop describes a property type.
type Prop interface {
	ID() ID
	SectionID() SectionID
}

var propRegistry = map[ID]Prop{}

func registerProp(p Prop) {
	id := p.ID()

	if _, ok := propRegistry[id]; ok {
		log.Panicf("ID collision for property %q", id)
	}

	propRegistry[id] = p
}

// EnumerateProperties enumerates all known global properties into a map of
func EnumerateProperties(defaultSection string) map[SectionID][]Prop {
	sectslug := SectionID(Slugify(defaultSection))
	enumerated := make(map[SectionID][]Prop)

	for _, prop := range propRegistry {
		sectionID := prop.SectionID()
		if sectionID == "" {
			sectionID = sectslug
		}

		enumerated[sectionID] = append(enumerated[sectionID], prop)
	}

	for _, props := range enumerated {
		sort.Slice(props, func(i, j int) bool {
			return sortutil.StrlessFold(
				string(props[i].ID()),
				string(props[j].ID()),
			)
		})
	}

	return enumerated
}
