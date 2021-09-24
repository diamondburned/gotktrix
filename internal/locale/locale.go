package locale

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type ctxKey uint

const (
	printerKey ctxKey = iota
)

var (
	localPrinter     *message.Printer
	localPrinterOnce sync.Once
)

func loadLocalPrinter() {
	localPrinterOnce.Do(func() {
		var langs []language.Tag

		for _, lang := range glib.GetLanguageNames() {
			t, err := language.Parse(lang)
			if err != nil {
				log.Printf("cannot parse language %s: %v", lang, err)
				continue
			}
			langs = append(langs, t)
		}

		// English fallback.
		if len(langs) < 1 {
			langs = append(langs, language.English)
		}

		localPrinter = message.NewPrinter(langs[0])
	})
}

// WithLocalPrinter inserts the local printer into the context scope.
func WithLocalPrinter(ctx context.Context) context.Context {
	loadLocalPrinter()
	return WithPrinter(ctx, localPrinter)
}

// WithPrinter inserts the given printer into a new context and  returns it.
func WithPrinter(ctx context.Context, p *message.Printer) context.Context {
	return context.WithValue(ctx, printerKey, p)
}

// Sprint calls ctx's message printer's Sprint.
func Sprint(ctx context.Context, a ...interface{}) string {
	return Printer(ctx).Sprint(a...)
}

// FromKey returns the formatted message from the given ID and fallback.
func FromKey(ctx context.Context, id, fallback string) string {
	return Sprint(ctx, message.Key(id, fallback))
}

// Sprintf calls ctx's message printer's Sprintf.
func Sprintf(ctx context.Context, k message.Reference, a ...interface{}) string {
	return Printer(ctx).Sprintf(k, a...)
}

// Printer returns the printer inside the context OR the local printer if none.
func Printer(ctx context.Context) *message.Printer {
	p, ok := ctx.Value(printerKey).(*message.Printer)
	if ok {
		return p
	}
	return localPrinter
}

// Plural formats the given list of strings and returns the strings joined (by
// commas) appended by either the singular case if len(v) == 1 or plural
// otherwise. If len(v) is 0, then an empty string is returned.
//
// TODO: deprecate this and replace it with something more locale-friendly,
// preferably using x/text/feature/plural.
func Plural(ctx context.Context, v []string, singular, plural string) string {
	p := Printer(ctx)

	singular = p.Sprint(" ") + p.Sprint(singular)
	plural = p.Sprint(" ") + p.Sprint(plural)

	switch len(v) {
	case 0:
		return ""
	case 1:
		return v[0] + singular
	case 2:
		return v[0] + p.Sprint(" and ") + v[1] + plural
	default:
		return strings.Join(v[:len(v)-1], p.Sprint(", ")) + p.Sprint(" and ") + v[len(v)-1] + plural
	}
}
