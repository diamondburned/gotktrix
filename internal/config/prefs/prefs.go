// Package prefs provides a publish-subscription API for global settinsg
// management. It exists as a schema-less version of the GSettings API.
package prefs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/locale"
	"github.com/diamondburned/gotktrix/internal/sortutil"
	"github.com/pkg/errors"
	"golang.org/x/text/message"
)

// propRegistry is the global registry map.
var propRegistry = map[ID]Prop{}

func registerProp(p Prop) {
	id := p.Meta().ID()

	if _, ok := propRegistry[id]; ok {
		log.Panicf("ID collision for property %q", id)
	}

	propRegistry[id] = p
}

// propOrder maps English prop names to the order integer.
type propOrder map[string]string

// sectionOrders maps a section name to all its prop orders.
var sectionOrders = map[string]propOrder{}

func OrderBefore(prop, isBefore Prop) {
	p := prop.Meta()
	b := isBefore.Meta()

	if p.EnglishSectionName() != b.EnglishSectionName() {
		panic("BUG: prop/before mismatch section name")
	}

	orders, ok := sectionOrders[p.EnglishSectionName()]
	if !ok {
		orders = propOrder{}
		sectionOrders[p.EnglishSectionName()] = orders
	}

	orders[p.EnglishName()] = b.EnglishName()
}

// Order registers the given names for ordering properties. It is only valid
// within the same sections.
func Order(props ...Prop) {
	for i, prop := range props[1:] {
		// slice where 1st item is popped off, so 1st is 2nd.
		OrderBefore(props[i], prop)
	}
}

// TODO: scrap this routine; just sort normally and scramble the slice
// afterwards.
func sectionPropOrder(orders propOrder, i, j string) bool {
	if orders != nil {
		// I honestly have no idea how I thought of this code. But I did.
		// https://go.dev/play/p/ONG4HbU_Rhl
		ibefore, iok := orders[i]
		jbefore, jok := orders[j]

		if iok && ibefore == j {
			return true
		}
		if jok && jbefore == i {
			return false
		}

		if iok {
			return sectionPropOrder(orders, ibefore, j)
		}
		if jok {
			return sectionPropOrder(orders, i, jbefore)
		}
	}

	return sortutil.LessFold(i, j)
}

// LoadData loads the given JSON data (usually returned from ReadSavedData)
// directly into the global preference values.
func LoadData(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var props map[string]json.RawMessage
	if err := json.Unmarshal(data, &props); err != nil {
		return err
	}
	for k, blob := range props {
		prop, ok := propRegistry[ID(k)]
		if !ok {
			continue
		}
		if err := prop.UnmarshalJSON(blob); err != nil {
			return fmt.Errorf("error at %s: %w", k, err)
		}
	}
	return nil
}

// Snapshot describes a snapshot of the preferences state.
type Snapshot map[string]json.RawMessage

// TakeSnapshot takes a snapshot of the global preferences into a flat map. This
// function should only be called on the main thread, but the returned snapshot
// can be used anywhere.
func TakeSnapshot() Snapshot {
	v := make(map[string]json.RawMessage, len(propRegistry))
	for id, prop := range propRegistry {
		b, err := prop.MarshalJSON()
		if err != nil {
			log.Panicf("cannot marshal property %q: %s", id, err)
		}
		v[string(id)] = json.RawMessage(b)
	}
	return v
}

// JSON marshals the snapshot as JSON. Any error that arises from marshaling the
// JSON is assumed to be the user tampering with it.
func (s Snapshot) JSON() []byte {
	b, err := json.Marshal(s)
	if err != nil {
		log.Panicln("prefs: cannot marshal snapshot:", err)
	}
	return b
}

var prefsPath = config.Path("prefs.json")

// Save atomically saves the snapshot to file.
func (s Snapshot) Save() error {
	tmp, err := os.CreateTemp(filepath.Dir(prefsPath), ".tmp.*")
	if err != nil {
		return errors.Wrap(err, "cannot mktemp")
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := tmp.Write(s.JSON()); err != nil {
		return errors.Wrap(err, "cannot write to temp file")
	}
	if err := tmp.Close(); err != nil {
		return errors.Wrap(err, "temp file error")
	}

	if err := os.Rename(tmp.Name(), prefsPath); err != nil {
		return errors.Wrap(err, "cannot swap new prefs file")
	}

	return nil
}

// ReadSavedData reads the saved preferences from a predetermined location.
// Users should give the returned byte slice to LoadData. A nil byte slice is a
// valid value.
func ReadSavedData() ([]byte, error) {
	b, err := os.ReadFile(prefsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		log.Println("cannot open prefs.json:", err)
		return nil, err
	}
	return b, nil
}

// ListedSection holds a list of properties returned from ListProperties.
type ListedSection struct {
	Name  string // localized
	Props []LocalizedProp

	name string // unlocalized
}

// LocalizedProp wraps Prop and localizes its name and description.
type LocalizedProp struct {
	Prop
	Name        string
	Description string
}

// ListProperties enumerates all known global properties into a map of
func ListProperties(ctx context.Context) []ListedSection {
	m := map[message.Reference][]Prop{}

	for _, prop := range propRegistry {
		meta := prop.Meta()
		m[meta.Section] = append(m[meta.Section], prop)
	}

	localize := locale.SFunc(ctx)

	sections := make([]ListedSection, 0, len(m))

	for s, props := range m {
		section := ListedSection{
			Name:  localize(s),
			Props: make([]LocalizedProp, len(props)),
			name:  props[0].Meta().EnglishSectionName(),
		}

		for i, prop := range props {
			section.Props[i] = LocalizedProp{
				Prop:        prop,
				Name:        localize(prop.Meta().Name),
				Description: localize(prop.Meta().Description),
			}
		}

		orders := sectionOrders[section.name]

		sort.Slice(section.Props, func(i, j int) bool {
			iname := section.Props[i].Meta().EnglishName()
			jname := section.Props[j].Meta().EnglishName()
			return sectionPropOrder(orders, iname, jname)
		})

		sections = append(sections, section)
	}

	sort.Slice(sections, func(i, j int) bool {
		return sortutil.LessFold(sections[i].name, sections[j].name)
	})

	return sections
}
