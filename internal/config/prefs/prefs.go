// Package prefs provides a publish-subscription API for global settinsg
// management. It exists as a schema-less version of the GSettings API.
package prefs

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/locale"
	"github.com/diamondburned/gotktrix/internal/sortutil"
	"github.com/pkg/errors"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
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

// PropMeta describes the metadata of a preference value.
type PropMeta struct {
	Name        message.Reference
	Section     message.Reference
	Description message.Reference
}

// Meta returns itself. It implements Prop.
func (p *PropMeta) Meta() *PropMeta { return p }

var nullPrinter = message.NewPrinter(
	language.Und,
	message.Catalog(message.DefaultCatalog),
)

// PropID implements Prop.
func (p *PropMeta) ID() ID {
	id := ID(Slugify(nullPrinter.Sprintf(p.Name)))
	id += ID(Slugify(nullPrinter.Sprintf(p.Section)))
	return id
}

func (p *PropMeta) validate() {
	// if p.ID == "" {
	// 	log.Panicln("missing prop ID")
	// }
	if p.Name == nil || p.Name == "" {
		log.Panicln("missing prop name")
	}
	if p.Section == nil || p.Section == "" {
		log.Panicln("missing prop section")
	}
}

// Prop describes a property type.
type Prop interface {
	Subscriber
	json.Marshaler
	json.Unmarshaler
	// Meta returns the property's meta.
	Meta() *PropMeta
}

// ErrInvalidAnyType is returned by a preference property if it has the wrong
// type.
var ErrInvalidAnyType = errors.New("incorrect value type")

var propRegistry = map[ID]Prop{}

func registerProp(p Prop) {
	id := p.Meta().ID()

	if _, ok := propRegistry[id]; ok {
		log.Panicf("ID collision for property %q", id)
	}

	propRegistry[id] = p
}

// Snapshot takes a snapshot of the global preferences into a flat map.
func Snapshot() map[string]interface{} {
	v := make(map[string]interface{}, len(propRegistry))
	for id, prop := range propRegistry {
		b, err := prop.MarshalJSON()
		if err != nil {
			log.Panicf("cannot marshal property %q: %s", id, err)
		}
		v[string(id)] = json.RawMessage(b)
	}
	return v
}

// MarshalJSON takes a snapshot of the preferences and marshals it into JSON.
func MarshalJSON() []byte {
	snapshot := Snapshot()
	b, err := json.Marshal(snapshot)
	if err != nil {
		log.Panicln("prefs: cannot marshal snapshot:", err)
	}
	return b
}

var filePath = config.Path("prefs.json")

// Save saves the config to file.
func Save() error {
	return os.WriteFile(filePath, MarshalJSON(), os.ModePerm)
}

// Load loads the config.
func Load() error {
	f, err := os.Open(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println("cannot open prefs.json:", err)
			return err
		}
		return nil
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&propRegistry); err != nil {
		return errors.Wrap(err, "prefs.json has invalid JSON")
	}

	return nil
}

// Section holds a list of properties.
type Section struct {
	Name  string // localized
	Props []LocalizedProp
}

// LocalizedProp wraps Prop and localizes its name and description.
type LocalizedProp struct {
	Prop
	Name        string
	Description string
}

// ListProperties enumerates all known global properties into a map of
func ListProperties(ctx context.Context) []Section {
	m := map[message.Reference][]Prop{}

	for _, prop := range propRegistry {
		meta := prop.Meta()
		m[meta.Section] = append(m[meta.Section], prop)
	}

	localize := locale.SFunc(ctx)

	var sections []Section

	for s, props := range m {
		section := Section{
			Name:  localize(s),
			Props: make([]LocalizedProp, len(props)),
		}
		for i, prop := range props {
			section.Props[i] = LocalizedProp{
				Prop:        prop,
				Name:        localize(prop.Meta().Name),
				Description: localize(prop.Meta().Description),
			}
		}
		sort.Slice(props, func(i, j int) bool {
			return sortutil.LessFold(
				string(props[i].Meta().ID()),
				string(props[j].Meta().ID()),
			)
		})
	}

	return sections
}
