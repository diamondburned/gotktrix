package autocomplete

import (
	"context"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/emojis"
	"github.com/diamondburned/gotktrix/internal/gotktrix/indexer"
	"github.com/diamondburned/gotrix/matrix"
	unicodeemoji "github.com/enescakir/emoji"
	"github.com/sahilm/fuzzy"
)

// Searcher is the interface for anything that can handle searching up a
// particular entity, such as a room member.
type Searcher interface {
	// Rune is the triggering rune for this searcher.
	Rune() rune
	// Search searches the given string and returns a list of data. The returned
	// list of Data only needs to be valid until the next call of Search.
	Search(ctx context.Context, str string) []Data
}

// Data represents a data structure capable of being displayed inside a list by
// constructing a new ListBoxRow.
type Data interface {
	// Row constructs a new ListBoxRow for display inside the list.
	Row(context.Context) *gtk.ListBoxRow
}

// dataList is for internal use only.
type dataList []Data

func (l *dataList) clear() {
	for i := range *l {
		(*l)[i] = nil
	}
	*l = (*l)[:0]
}

func (l *dataList) add(data Data) {
	*l = append(*l, data)
}

// RoomMemberData is the data for each room member. It implements Data.
type RoomMemberData indexer.IndexedRoomMember

var subNameAttrs = textutil.Attrs(
	pango.NewAttrScale(0.85),
	pango.NewAttrForegroundAlpha(75*65535/100), // 75%
)

// Row implements Data.
func (d RoomMemberData) Row(ctx context.Context) *gtk.ListBoxRow {
	client := gotktrix.FromContext(ctx).Offline()
	author := mauthor.Markup(client, d.Room, d.ID,
		mauthor.WithWidgetColor(),
		mauthor.WithMinimal(),
	)

	name := gtk.NewLabel("")
	name.SetMarkup(author)
	name.SetEllipsize(pango.EllipsizeEnd)
	name.SetXAlign(0)

	sub := gtk.NewLabel(string(d.ID))
	sub.SetXAlign(0)
	sub.SetAttributes(subNameAttrs)

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(name)
	box.Append(sub)

	row := gtk.NewListBoxRow()
	row.SetChild(box)
	row.AddCSSClass("autocomplete-member")

	return row
}

// NewRoomMemberSearcher creates a new searcher constructor that can search up
// room members for the given room. It matches using '@'.
func NewRoomMemberSearcher(ctx context.Context, roomID matrix.RoomID) Searcher {
	return &roomMemberSearcher{
		rms: gotktrix.FromContext(ctx).Index.SearchRoomMember(roomID, MaxResults),
		res: make([]Data, 0, MaxResults),
	}
}

type roomMemberSearcher struct {
	rms indexer.RoomMemberSearcher
	res dataList
}

func (s *roomMemberSearcher) Rune() rune { return '@' }

func (s *roomMemberSearcher) Search(ctx context.Context, str string) []Data {
	// Refuse to search if the user hasn't inputted more than 2 characters. We
	// have to do this because Bleve will freeze the client slightly otherwise.
	if len(str) < 2 {
		return nil
	}

	results := s.rms.Search(ctx, str)
	if len(results) == 0 {
		return nil
	}

	s.res.clear()
	for _, result := range results {
		s.res.add(RoomMemberData(result))
	}

	return s.res
}

// NewEmojiSearcher creates a new emoji searcher.
func NewEmojiSearcher(ctx context.Context, roomID matrix.RoomID) Searcher {
	return &emojiSearcher{
		client: gotktrix.FromContext(ctx),
		roomID: roomID,
		res:    make(dataList, 0, MaxResults),
	}
}

type emojiSearcher struct {
	client *gotktrix.Client
	roomID matrix.RoomID

	res dataList

	emotes  map[emojis.EmojiName]emojis.Emoji
	matches []string
	updated time.Time
	// fetched bool
}

func (s *emojiSearcher) Rune() rune { return ':' }

const cacheExpiry = time.Minute

func (s *emojiSearcher) update() {
	now := time.Now()
	if s.matches != nil && s.updated.Add(cacheExpiry).After(now) {
		return
	}

	s.updated = now

	userEmotes, _ := emojis.UserEmotes(s.client.Offline())
	roomEmotes, _ := emojis.RoomEmotes(s.client.Offline(), s.roomID)

	if len(userEmotes)+len(roomEmotes) == 0 {
		s.emotes = nil
		return
	}

	// It's likely cheaper to just reallocate the emojis object if the length
	// does not match. We can allow some cache inconsistency; it doesn't impact
	// that significantly.
	if t := len(userEmotes) + len(roomEmotes); t != len(s.emotes) {
		s.emotes = make(map[emojis.EmojiName]emojis.Emoji, t)
	}

	// Keep track of changes, so we can reconstruct the matcher object if
	// needed.
	var changed bool

	// Prioritize user emotes over room emotes.
	for name, emote := range userEmotes {
		if _, ok := s.emotes[name]; ok {
			continue
		}
		s.emotes[name] = emote
		changed = true
	}

	for name, emote := range roomEmotes {
		if _, ok := s.emotes[name]; ok {
			continue
		}
		s.emotes[name] = emote
		changed = true
	}

	if changed {
		s.updateMatches()
	}
}

var unicodeEmojis = unicodeemoji.Map()

// updateMatches is fairly expensive!
func (s *emojiSearcher) updateMatches() {
	if s.matches == nil {
		s.matches = make([]string, 0, len(s.emotes)+len(unicodeEmojis))
		for name := range unicodeEmojis {
			s.matches = append(s.matches, name)
		}
	} else {
		s.matches = s.matches[:len(unicodeEmojis)]
	}

	for name := range s.emotes {
		s.matches = append(s.matches, string(name))
	}
}

func (s *emojiSearcher) Search(ctx context.Context, str string) []Data {
	s.update()
	s.res.clear()

	matches := fuzzy.Find(str, s.matches)
	if len(matches) == 0 {
		return nil
	}

	if len(matches) > MaxResults {
		matches = matches[:MaxResults]
	}

	for _, match := range matches {
		d := EmojiData{Name: match.Str}

		if custom, ok := s.emotes[emojis.EmojiName(d.Name)]; ok {
			d.Custom = custom
			goto gotData
		}

		if u, ok := unicodeEmojis[d.Name]; ok {
			d.Unicode = u
			goto gotData
		}

		continue
	gotData:
		s.res.add(d)
	}

	return s.res
}

// EmojiData is the Data structure for each emoji.
type EmojiData struct {
	Name string

	// either or
	Unicode string
	Custom  emojis.Emoji
}

const emojiSize = 32 // px

var _ = cssutil.WriteCSS(`
	.autocompleter-unicode {
		font-size: 26px;
	}
`)

func (d EmojiData) Row(ctx context.Context) *gtk.ListBoxRow {
	b := gtk.NewBox(gtk.OrientationHorizontal, 4)

	if d.Unicode != "" {
		l := gtk.NewLabel(d.Unicode)
		l.AddCSSClass("autocompleter-unicode")

		b.Append(l)
	} else {
		i := gtk.NewImage()
		i.AddCSSClass("autocompleter-custom")
		i.SetSizeRequest(emojiSize, emojiSize)

		client := gotktrix.FromContext(ctx).Offline()
		url, _ := client.SquareThumbnail(d.Custom.URL, emojiSize, gtkutil.ScaleFactor())
		// Use a background context so we don't constantly thrash the server
		// with cancelled requests every time we time.
		imgutil.AsyncGET(ctx, url, imgutil.ImageSetterFromImage(i))

		b.Append(i)
	}

	l := gtk.NewLabel(d.Name)
	l.SetMaxWidthChars(35)
	l.SetEllipsize(pango.EllipsizeMiddle)
	b.Append(l)

	r := gtk.NewListBoxRow()
	r.AddCSSClass("autocomplete-emoji")
	r.SetChild(b)

	return r
}
