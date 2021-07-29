package state

import (
	"encoding/binary"
	"log"

	"github.com/chanbakjsd/gotrix/api"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/db"
)

type dbPaths struct {
	user      db.NodePath
	rooms     db.NodePath
	timelines db.NodePath
}

func newDBPaths(topPath db.NodePath) dbPaths {
	return dbPaths{
		user:      topPath.Tail("user"),
		rooms:     topPath.Tail("rooms"),
		timelines: topPath.Tail("timelines"),
	}
}

func setRawEvent(n db.Node, roomID matrix.RoomID, raw *event.RawEvent) {
	raw.RoomID = roomID

	var dbKey string
	if raw.StateKey != "" {
		dbKey = db.Keys(string(raw.Type), string(raw.StateKey))
	} else {
		dbKey = string(raw.Type)
	}

	if err := n.Set(dbKey, raw); err != nil {
		log.Printf("failed to set Matrix event for room %q: %v", roomID, err)
	}
}

func (p *dbPaths) setRaws(n db.Node, roomID matrix.RoomID, raws []event.RawEvent) {
	if roomID != "" {
		n = n.FromPath(p.rooms.Tail(string(roomID)))
	} else {
		n = n.FromPath(p.user)
	}

	for i := range raws {
		setRawEvent(n, roomID, &raws[i])
	}
}

func (p *dbPaths) setStrippeds(n db.Node, roomID matrix.RoomID, raws []event.StrippedEvent) {
	if roomID != "" {
		n = n.FromPath(p.rooms.Tail(string(roomID)))
	} else {
		n = n.FromPath(p.user)
	}

	for i := range raws {
		setRawEvent(n, roomID, &raws[i].RawEvent)
	}
}

func (p *dbPaths) timelinePath(roomID matrix.RoomID) db.NodePath {
	return p.timelines.Tail(string(roomID))
}

func (p *dbPaths) timelineEventsPath(roomID matrix.RoomID) db.NodePath {
	return p.timelines.Tail(string(roomID), "events")
}

// timelineEventKey formats a key that the internal database can use to sort the
// returned values lexicographically.
func timelineEventKey(ev *event.RawEvent) string {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(ev.OriginServerTime))
	// Hopefully this is never printed.
	return string(b) + "\x00" + string(ev.ID)
}

func (p *dbPaths) setTimeline(n db.Node, roomID matrix.RoomID, tl api.SyncTimeline) {
	tnode := n.FromPath(p.timelineEventsPath(roomID))

	for i := range tl.Events {
		key := timelineEventKey(&tl.Events[i])

		if err := tnode.Set(key, &tl.Events[i]); err != nil {
			log.Printf("failed to set Matrix timeline event for room %q: %v", roomID, err)
		}
	}

	// Clean up the timeline events.
	if err := tnode.DropExceptLast(TimelineKeepLast); err != nil {
		log.Printf("failed to clean up Matrix timeline for room %q: %v", roomID, err)
	}

	// Write the previous batch string, if any.
	if tl.PreviousBatch != "" {
		rnode := n.FromPath(p.timelinePath(roomID))

		if err := rnode.Set("previous_batch", tl.PreviousBatch); err != nil {
			log.Printf("failed to set previous_batch for room %q: %v", roomID, err)
		}
	}
}

func (p *dbPaths) deleteTimeline(n db.Node, roomID matrix.RoomID) {
	n = n.FromPath(p.timelinePath(roomID))

	if err := n.Drop(); err != nil {
		log.Printf("failed to delete Matrix timeline for room %q: %v", roomID, err)
	}
}
