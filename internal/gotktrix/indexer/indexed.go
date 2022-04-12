package indexer

import (
	"github.com/blevesearch/bleve/v2"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

// func doc(id string, fields []document.Field) {}

// IndexedRoomMember is the data structure representing indexed room member
// information.
type IndexedRoomMember struct {
	ID   matrix.UserID `json:"id"`
	Room matrix.RoomID `json:"room_id"`
	Name string        `json:"name"`
}

func indexRoomMember(m *event.RoomMemberEvent) IndexedRoomMember {
	idx := IndexedRoomMember{
		ID:   m.UserID,
		Room: m.RoomID,
	}
	if m.DisplayName != nil {
		idx.Name = *m.DisplayName
	}
	return idx
}

// Index indexes m into the given Bleve indexer.
func (m *IndexedRoomMember) Index(b *bleve.Batch) error {
	// b.IndexAdvanced(document.NewDocument)
	return b.Index(string(m.Room)+"\x01"+string(m.ID), m)
}

// Type returns RoomMember.
func (m *IndexedRoomMember) Type() string {
	return "RoomMember"
}
