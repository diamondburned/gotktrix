package hl

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
)

var Style = prefs.NewString("", prefs.StringMeta{
	Name:    "Code Highlight Style",
	Section: "Text",
	Description: "For reference, see the " +
		`<a href="https://xyproto.github.io/splash/docs/all.html">Chroma Style Gallery</a>.`,
	Placeholder: "Leave blank for default",
	Validate: func(style string) error {
		// _, err := findStyle(style)
		// return err
		return nil
	},
})

// Styles used if the user hasn't set a style in the config.
var (
	DefaultDarkStyle  = styles.Monokai
	DefaultLightStyle = styles.SolarizedLight
)

// tagMap is a map from chroma token types to text tag attributes.
type tagMap map[chroma.TokenType]textutil.TextTag

var defaultStyles struct {
	darkMap   tagMap
	lightMap  tagMap
	darkOnce  sync.Once
	lightOnce sync.Once
}

func defaultTagMap(darkThemed bool) tagMap {
	if darkThemed {
		return darkThemeTagMap()
	} else {
		return lightThemeTagMap()
	}
}

func darkThemeTagMap() tagMap {
	defaultStyles.darkOnce.Do(func() {
		defaultStyles.darkMap = convertStyle(DefaultDarkStyle)
	})
	return defaultStyles.darkMap
}

func lightThemeTagMap() tagMap {
	defaultStyles.lightOnce.Do(func() {
		defaultStyles.lightMap = convertStyle(DefaultLightStyle)
	})
	return defaultStyles.lightMap
}

var (
	lexerMu    sync.Mutex
	lexerCache sync.Map
)

func loadStyle(ctx context.Context, theme string) *chroma.Style {
	s := styles.Get(theme)
	if s != styles.Fallback {
		return s
	}

	app := app.FromContext(ctx)
	if theme == "" || app == nil {
		return styles.Fallback
	}

	d, err := os.ReadFile(app.ConfigPath("styles", theme+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("hl: unknown style %s", theme)
		} else {
			log.Printf("hl: error reading style %s.json: %v", theme, err)
		}
		return styles.Fallback
	}

	var style chroma.StyleEntries
	if err := json.Unmarshal(d, &style); err != nil {
		log.Printf("hl: failed to parse JSON style %s.json: %v", theme, err)
		return styles.Fallback
	}

	s, err = chroma.NewStyle(theme, style)
	if err != nil {
		log.Printf("hl: failed to parse actual styling for %s.json: %v", theme, err)
		return styles.Fallback
	}

	return s
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

func styleEntryToTag(e chroma.StyleEntry) textutil.TextTag {
	attrs := make(textutil.TextTag, 5)

	if e.Colour.IsSet() {
		attrs["foreground"] = e.Colour.String()
	}
	if e.Border.IsSet() {
		attrs["background"] = e.Border.String()
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

// ChangeStyle changes the global highlighter style. It is a helper function for
// the Style variable.
func ChangeStyle(styleName string) error {
	return Style.Publish(styleName)
}

const hlPrefix = "_hl_"

// Highlight highlights the code section starting from start to end using the
// lexer of the given language. The start and end iterators will be invalidated,
// but the end iterator will have its previous offset restored.
func Highlight(ctx context.Context, start, end *gtk.TextIter, language string) {
	// Store this so we can restore it.
	endOffset := end.Offset()

	// Memorize the offset of the end iterator.
	buf := start.Buffer()
	txt := buf.Slice(start, end, true)

	lexer := lexer(language)

	i, err := lexer.Tokenise(nil, txt)
	if err != nil {
		return
	}

	f := newFormatter(ctx, buf, start, end, language)
	f.resetTags()
	f.do(i)
	f.discard()

	end.SetOffset(endOffset)
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
		return &state{
			ranges: make([][2]int, 0, 10),
		}
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

	var tokenTags tagMap
	if style := Style.Value(); style != "" {
		tokenTags = convertStyle(loadStyle(ctx, style))
	} else {
		tokenTags = defaultTagMap(textutil.IsDarkTheme())
	}

	return formatter{
		buf:       buf,
		tags:      buf.TagTable(),
		start:     start,
		end:       end,
		tokenTags: tokenTags,
		state:     statePool.Get().(*state),
	}
}

func (f *formatter) discard() {
	f.state.discard()
	*f = formatter{}
}

func (f *formatter) resetTags() {
	removeTags := make([]*gtk.TextTag, 0, f.tags.Size())

	f.tags.ForEach(func(tag *gtk.TextTag) {
		if strings.HasPrefix(tag.ObjectProperty("name").(string), hlPrefix) {
			removeTags = append(removeTags, tag)
		}
	})

	for _, tag := range removeTags {
		f.buf.RemoveTag(tag, f.start, f.end)
	}
}

func (f *formatter) do(iter chroma.Iterator) {
	offset := f.start.Offset()

	for _, token := range iter.Tokens() {
		end := offset + utf8.RuneCountInString(token.Value)
		tag := f.tag(token.Type)

		if tag != nil {
			f.start.SetOffset(offset)
			f.end.SetOffset(end)
			f.buf.ApplyTag(tag, f.start, f.end)
		}

		offset = end
	}
}

func (f *formatter) tag(tt chroma.TokenType) *gtk.TextTag {
	attrs := f.tagAttrs(tt)
	if attrs == nil {
		return nil
	}

	tname := hlPrefix + attrs.Hash()

	if tag := f.tags.Lookup(tname); tag != nil {
		return tag
	}

	tag := attrs.Tag(tname)
	f.tags.Add(tag)

	return tag
}

func (f *formatter) tagAttrs(tt chroma.TokenType) textutil.TextTag {
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
