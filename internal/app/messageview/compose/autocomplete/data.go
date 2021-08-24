package autocomplete

import (
	"context"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/indexer"
	"github.com/diamondburned/gotktrix/internal/gtkutil/markuputil"
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
	Row() *gtk.ListBoxRow
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
type RoomMemberData struct {
	indexer.IndexedRoomMember
	ctx context.Context
}

var subNameAttrs = markuputil.Attrs(
	pango.NewAttrScale(0.85),
	pango.NewAttrForegroundAlpha(75*65535/100), // 75%
)

// Row implements Data.
func (d RoomMemberData) Row() *gtk.ListBoxRow {
	client := gotktrix.FromContext(d.ctx).Offline()
	author := mauthor.Markup(client, d.Room, d.ID,
		mauthor.WithWidgetColor(&app.Window(d.ctx).Widget),
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
		rms: gotktrix.FromContext(ctx).Index.SearchRoomMember(roomID),
		res: make([]Data, 0, indexer.QuerySize),
	}
}

type roomMemberSearcher struct {
	rms indexer.RoomMemberSearcher
	res dataList
}

func (s *roomMemberSearcher) Rune() rune { return '@' }

func (s *roomMemberSearcher) Search(ctx context.Context, str string) []Data {
	results := s.rms.Search(ctx, str)
	if len(results) == 0 {
		return nil
	}

	s.res.clear()
	for _, result := range results {
		s.res.add(RoomMemberData{
			IndexedRoomMember: result,
			ctx:               ctx,
		})
	}

	return s.res
}
