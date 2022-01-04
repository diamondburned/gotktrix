package locale

import (
	"context"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type ctxKey uint

const (
	printerKey ctxKey = iota
)

// NewLocalPrinter creates a new printer from the given options, with the
// language tags taken from the system's locales using GLib's API.
func NewLocalPrinter(opts ...message.Option) *message.Printer {
	var langs []language.Tag

	for _, lang := range glib.GetLanguageNames() {
		if lang == "C" {
			continue
		}

		icuLocale := strings.SplitN(lang, ".", 2)[0]

		t, err := language.Parse(icuLocale)
		if err == nil {
			langs = append(langs, t)
		}
	}

	// English fallback.
	if len(langs) < 1 {
		langs = append(langs, language.English)
	}

	return message.NewPrinter(langs[0], opts...)
}

// WithPrinter inserts the given printer into a new context and  returns it.
func WithPrinter(ctx context.Context, p *message.Printer) context.Context {
	return context.WithValue(ctx, printerKey, p)
}

// S returns the translated string from the given reference.
func S(ctx context.Context, a message.Reference) string {
	return FromContext(ctx).Sprint(a)
}

// SFunc is a helper function that wraps the given context to format multiple
// strings in a shorter syntax.
func SFunc(ctx context.Context) func(a message.Reference) string {
	p := FromContext(ctx)
	return func(a message.Reference) string { return p.Sprint(a) }
}

// Sprint calls ctx's message printer's Sprint.
func Sprint(ctx context.Context, a ...message.Reference) string {
	vs := make([]interface{}, len(a))
	for i, v := range a {
		vs[i] = v
	}
	return FromContext(ctx).Sprint(vs...)
}

// Sprintf calls ctx's message printer's Sprintf.
func Sprintf(ctx context.Context, k message.Reference, a ...interface{}) string {
	return FromContext(ctx).Sprintf(k, a...)
}

// Plural formats the string in plural form.
func Plural(ctx context.Context, one, many message.Reference, n int) string {
	// I don't know how x/text/plural works.
	p := FromContext(ctx)
	if n == 1 {
		return p.Sprintf(one, n)
	}
	return p.Sprintf(many, n)
}

// Printer is a message printer.
type Printer = message.Printer

// FromContext returns the printer inside the context or nil.
func FromContext(ctx context.Context) *Printer {
	p, ok := ctx.Value(printerKey).(*Printer)
	if ok {
		return p
	}
	return nil
}

// doubleSpaceCollider is used for some formatted timestamps to get rid of
// padding spaces.
var doubleSpaceCollider = strings.NewReplacer("  ", " ")

// Time formats the given timestamp as a locale-compatible timestamp.
func Time(t time.Time, long bool) string {
	glibTime := glib.NewDateTimeFromGo(t.Local())

	if long {
		return doubleSpaceCollider.Replace(glibTime.Format("%c"))
	}

	return glibTime.Format("%X")
}

const (
	Day  = 24 * time.Hour
	Week = 7 * Day
	Year = 365 * Day
)

type truncator struct {
	d time.Duration
	s message.Reference
}

var longTruncators = []truncator{
	{d: Day, s: "Today at %X"},
	{d: Week, s: "Monday at %X"},
	{d: -1, s: "%X %x"},
}

// TimeAgo formats a long string that expresses the relative time difference
// from now until t.
func TimeAgo(ctx context.Context, t time.Time) string {
	t = t.Local()

	trunc := t
	now := time.Now().Local()

	for _, truncator := range longTruncators {
		trunc = trunc.Truncate(truncator.d)
		now = now.Truncate(truncator.d)

		if trunc.Equal(now) || truncator.d == -1 {
			glibTime := glib.NewDateTimeFromGo(t)
			return glibTime.Format(S(ctx, truncator.s))
		}
	}

	return ""
}
