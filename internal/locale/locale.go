package locale

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type ctxKey uint

const (
	printerKey ctxKey = iota
)

var (
	localPrinter *message.Printer
	initOnce     sync.Once
)

func initialize() {
	initOnce.Do(func() {
		var langs []language.Tag

		for _, lang := range glib.GetLanguageNames() {
			if lang == "C" {
				continue
			}

			icuLocale := strings.SplitN(lang, ".", 2)[0]

			t, err := language.Parse(icuLocale)
			if err != nil {
				log.Printf("cannot parse language %s: %v", lang, err)
			} else {
				langs = append(langs, t)
			}
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
	initialize()
	return WithPrinter(ctx, localPrinter)
}

// WithPrinter inserts the given printer into a new context and  returns it.
func WithPrinter(ctx context.Context, p *message.Printer) context.Context {
	return context.WithValue(ctx, printerKey, p)
}

// S returns the translated string from the given reference.
func S(ctx context.Context, a message.Reference) string {
	return Printer(ctx).Sprint(a)
}

// SFunc is a helper function that wraps the given context to format multiple
// strings in a shorter syntax.
func SFunc(ctx context.Context) func(a message.Reference) string {
	p := Printer(ctx)
	return func(a message.Reference) string { return p.Sprint(a) }
}

// Sprint calls ctx's message printer's Sprint.
func Sprint(ctx context.Context, a ...message.Reference) string {
	vs := make([]interface{}, len(a))
	for i, v := range a {
		vs[i] = v
	}
	return Printer(ctx).Sprint(vs...)
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

// Time formats the given timestamp as a locale-compatible timestamp. Nothing is
// actually locale-friendly yet, though.
func Time(t time.Time, long bool) string {
	glibTime := glib.NewDateTimeFromGo(t)

	if long {
		return glibTime.Format("%c")
	}

	return glibTime.Format("%X")
}
