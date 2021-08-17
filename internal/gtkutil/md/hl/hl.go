package hl

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/config/prefs"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
	"github.com/pkg/errors"
)

// tagMap is a map from chroma token types to text tag attributes.
type tagMap map[chroma.TokenType]markuputil.TextTag

var (
	lexerMu    sync.Mutex
	lexerCache sync.Map
)

var (
	globalMu sync.RWMutex
	global   tagMap
)

var stylePath = config.Path("styles")

var Style = prefs.NewString("", prefs.StringMeta{
	PropMeta: prefs.PropMeta{
		Name:        "Highlight Style",
		Section:     "Appearance",
		Description: "For reference, see https://xyproto.github.io/splash/docs/all.html.",
	},
	Validate: func(style string) error {
		_, err := findStyle(style)
		return err
	},
})

func init() {
	s, err := findStyle(Style.Value())
	if err != nil {
		log.Panicln("hl: failed to parse default style:", err)
	}

	if s != styles.Fallback {
		global = convertStyle(s)
	}
}

func findStyle(theme string) (*chroma.Style, error) {
	s := styles.Get(theme)
	if s != styles.Fallback {
		return s, nil
	}

	if theme == "" {
		return styles.Fallback, nil
	}

	d, err := os.ReadFile(filepath.Join(stylePath, theme+".json"))
	if err != nil {
		return nil, err
	}

	var styles chroma.StyleEntries
	if err := json.Unmarshal(d, &styles); err != nil {
		return nil, errors.Wrap(err, "failed to parse JSON chroma styles")
	}

	s, err = chroma.NewStyle(theme, styles)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse actual styling")
	}

	return s, nil
}

func convertStyle(style *chroma.Style) tagMap {
	tags := make(tagMap)
	bg := style.Get(chroma.Background)

	for t := range chroma.StandardTypes {
		entry := style.Get(t)
		// Remove the theme's default background.
		if t != chroma.Background {
			entry = entry.Sub(bg)
		}

		if entry.IsZero() {
			continue
		}

		tags[t] = styleEntryToTag(entry)
	}

	return tags
}

func styleEntryToTag(e chroma.StyleEntry) markuputil.TextTag {
	attrs := make(markuputil.TextTag, 3)

	if e.Colour.IsSet() {
		attrs["foreground"] = e.Colour.String()
	}
	if e.Background.IsSet() {
		attrs["background"] = e.Background.String()
	}
	if e.Bold == chroma.Yes {
		attrs["weight"] = pango.WeightBold
	}
	if e.Italic == chroma.Yes {
		attrs["style"] = pango.StyleItalic
	}
	if e.Underline == chroma.Yes {
		attrs["underline"] = pango.UnderlineSingle
	}

	return attrs
}

// Styles used if the user hasn't set a style in the config.
const (
	DefaultDarkStyle  = "monokai"
	DefaultLightStyle = "solarized-light"
)

var (
	darkStyleMap  tagMap
	darkStyleOnce sync.Once

	lightStyleMap  tagMap
	lightStyleOnce sync.Once
)

func defaultStyle(dark bool) tagMap {
	if dark {
		darkStyleOnce.Do(func() {
			s, err := findStyle(DefaultDarkStyle)
			if err != nil {
				log.Println("hl: built-in dark style", DefaultDarkStyle, "not found")
				s = styles.Fallback
			}
			darkStyleMap = convertStyle(s)
		})

		return darkStyleMap
	} else {
		lightStyleOnce.Do(func() {
			s, err := findStyle(DefaultLightStyle)
			if err != nil {
				log.Println("hl: built-in dark style", DefaultLightStyle, "not found")
				s = styles.Fallback
			}
			lightStyleMap = convertStyle(s)
		})

		return lightStyleMap
	}
}

// ChangeStyle changes the global highlighter style. It is a helper function for
// the Style variable.
func ChangeStyle(styleName string) error {
	return Style.Publish(styleName)
}

// Highlight highlights the code section starting from start to end using the
// lexer of the given language. The start and end iterators will be invalidated
// to undefined positions inbetween start and end.
func Highlight(ctx context.Context, start, end *gtk.TextIter, language string) {
	buf := start.Buffer()
	txt := buf.Slice(start, end, true)

	lexer := lexer(language)

	i, err := lexer.Tokenise(nil, txt)
	if err != nil {
		return
	}

	f := newFormatter(ctx, buf, start, end, language)
	f.do(i)
	f.discard()
}

func lexer(lang string) chroma.Lexer {
	v, ok := lexerCache.Load(lang)
	if ok {
		return v.(chroma.Lexer)
	}

	lexerMu.Lock()
	defer lexerMu.Unlock()

	// Recheck the cache before loading.
	v, ok = lexerCache.Load(lang)
	if ok {
		return v.(chroma.Lexer)
	}

	lexer := lexers.Get(lang)
	if lexer != nil {
		lexerCache.Store(lang, lexer)
		return lexer
	}

	return lexers.Fallback
}

// Formatter that generates Pango markup.
type formatter struct {
	buf  *gtk.TextBuffer
	tags *gtk.TextTagTable

	start *gtk.TextIter
	end   *gtk.TextIter // preallocated temp iter

	tokenTags tagMap
	state     *state
}

type state struct {
	ranges [][2]int
}

var statePool = sync.Pool{
	New: func() interface{} {
		state := &state{}
		state.ranges = make([][2]int, 0, 10)
		return state
	},
}

func (s *state) discard() {
	s.ranges = s.ranges[:0]
	statePool.Put(s)
}

// useState takes a state (possibly a new one) from the state pool.
func newFormatter(
	ctx context.Context,
	buf *gtk.TextBuffer, start, end *gtk.TextIter, lang string) formatter {

	globalMu.RLock()
	tokenTags := global
	globalMu.RUnlock()

	if tokenTags == nil {
		window := app.FromContext(ctx).Window()
		isDark := markuputil.IsDarkTheme(&window.Widget)
		tokenTags = defaultStyle(isDark)
	}

	f := formatter{
		buf:       buf,
		start:     start,
		end:       end,
		tokenTags: tokenTags,
		state:     statePool.Get().(*state),
	}
	f.tags = f.buf.TagTable()
	return f
}

func (f *formatter) discard() {
	f.state.discard()
	*f = formatter{}
}

func (f *formatter) do(iter chroma.Iterator) {
	offset := f.start.Offset()

	for _, token := range iter.Tokens() {
		end := offset + len(token.Value)
		attr := f.tagAttrs(token.Type)

		if attr != nil {
			// TODO: assess anonymous tags vs. hashed tags.
			tag := attr.Tag("")
			f.tags.Add(tag)

			f.start.SetOffset(offset)
			f.end.SetOffset(end)

			f.buf.ApplyTag(tag, f.start, f.end)
		}

		offset = end
	}
}

func (f *formatter) tagAttrs(tt chroma.TokenType) markuputil.TextTag {
	c, ok := f.tokenTags[tt]
	if ok {
		return c
	}

	tt = tt.SubCategory()
	c, ok = f.tokenTags[tt]
	if ok {
		return c
	}

	tt = tt.Category()
	c, ok = f.tokenTags[tt]
	if ok {
		return c
	}

	return nil
}
