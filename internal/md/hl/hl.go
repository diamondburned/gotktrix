package hl

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

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

var stylePath = config.Path("styles")

var Style = prefs.NewString("", prefs.StringMeta{
	Name:    "Code Highlight Style",
	Section: "Text",
	Description: "For reference, see the " +
		`<a href="https://xyproto.github.io/splash/docs/all.html">Chroma Style Gallery</a>.`,
	Placeholder: "Leave blank for default",
	Validate: func(style string) error {
		_, err := findStyle(style)
		return err
	},
})

func init() {
	Style.SubscribeInit(updateGlobal)
	// Realistically the user will never see it when the thing isn't
	// initialized, anyway.
	go updateGlobal()
}

// Styles used if the user hasn't set a style in the config.
const (
	DefaultDarkStyle  = "monokai"
	DefaultLightStyle = "solarized-light"
)

// tagMap is a map from chroma token types to text tag attributes.
type tagMap map[chroma.TokenType]markuputil.TextTag

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
		s, err := findStyle(DefaultDarkStyle)
		if err != nil {
			log.Println("hl: built-in dark style", DefaultDarkStyle, "not found")
			s = styles.Fallback
		}
		defaultStyles.darkMap = convertStyle(s)
	})
	return defaultStyles.darkMap
}

func lightThemeTagMap() tagMap {
	defaultStyles.lightOnce.Do(func() {
		s, err := findStyle(DefaultLightStyle)
		if err != nil {
			log.Println("hl: built-in dark style", DefaultLightStyle, "not found")
			s = styles.Fallback
		}
		defaultStyles.lightMap = convertStyle(s)
	})
	return defaultStyles.lightMap
}

var (
	lexerMu    sync.Mutex
	lexerCache sync.Map
)

var globalTag struct {
	sync.RWMutex
	tags tagMap
}

func updateGlobal() {
	s, err := findStyle(Style.Value())
	if err != nil {
		log.Panicln("hl: failed to parse default style:", err)
	}

	globalTag.Lock()
	if s == styles.Fallback {
		globalTag.tags = nil
	} else {
		globalTag.tags = convertStyle(s)
	}
	globalTag.Unlock()
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
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("unknown style %s", theme)
		}
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
	attrs := make(markuputil.TextTag, 5)

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

	globalTag.RLock()
	tokenTags := globalTag.tags
	globalTag.RUnlock()

	if tokenTags == nil {
		isDark := markuputil.IsDarkTheme(app.GTKWindowFromContext(ctx))
		tokenTags = defaultTagMap(isDark)
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
