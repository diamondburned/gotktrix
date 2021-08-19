package state

import (
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/chanbakjsd/gotrix/api"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/db"
)

type dbPaths struct {
	user      db.NodePath
	rooms     db.NodePath
	directs   db.NodePath
	summaries db.NodePath
	timelines db.NodePath
}

func newDBPaths(topPath db.NodePath) dbPaths {
	return dbPaths{
		user:      topPath.Tail("user"),
		rooms:     topPath.Tail("rooms"),
		directs:   topPath.Tail("directs"),
		summaries: topPath.Tail("summaries"),
		timelines: topPath.Tail("timelines"),
	}
}

func setRawEvent(n db.Node, roomID matrix.RoomID, raw *event.RawEvent, state bool) {
	raw.RoomID = roomID

	var dbKey string
	if raw.StateKey != "" {
		dbKey = db.Keys(string(raw.Type), string(raw.StateKey))
	} else {
		dbKey = string(raw.Type)
	}

	var err error
	if state {
		err = n.Set(dbKey, raw)
	} else {
		err = n.SetIfNone(dbKey, raw)
	}

	if err != nil {
		log.Printf("failed to set Matrix event for room %q: %v", roomID, err)
	}
}

func (p *dbPaths) setRaws(
	n db.Node, roomID matrix.RoomID, raws []event.RawEvent, state bool) {

	if roomID != "" {
		n = n.FromPath(p.rooms.Tail(string(roomID)))
	} else {
		n = n.FromPath(p.user)
	}

	for i := range raws {
		setRawEvent(n, roomID, &raws[i], state)
	}
}

func (p *dbPaths) setStrippeds(
	n db.Node, roomID matrix.RoomID, raws []event.StrippedEvent, state bool) {

	if roomID != "" {
		n = n.FromPath(p.rooms.Tail(string(roomID)))
	} else {
		n = n.FromPath(p.user)
	}

	for i := range raws {
		setRawEvent(n, roomID, &raws[i].RawEvent, state)
	}
}

func (p *dbPaths) setSummary(n db.Node, roomID matrix.RoomID, s api.SyncRoomSummary) {
	if roomID == "" {
		return // unexpecting
	}

	if err := n.FromPath(p.summaries).Set(string(roomID), &s); err != nil {
		log.Printf("failed to set Matrix room summary for room %q: %v", roomID, err)
	}
}

func (p *dbPaths) timelinePath(roomID matrix.RoomID) db.NodePath {
	return p.timelines.Tail(string(roomID))
}

func (p *dbPaths) timelineEventsPath(roomID matrix.RoomID) db.NodePath {
	return p.timelines.Tail(string(roomID), "events")
}

var i64ZeroPadding = func() string {
	v := len(strconv.FormatInt(math.MaxInt64, 32))
	return strings.Repeat("0", v)
}()

// timelineEventKey formats a key that the internal database can use to sort the
// returned values lexicographically.
func timelineEventKey(ev *event.RawEvent) string {
	str := strconv.FormatInt(int64(ev.OriginServerTime), 32)
	// Pad the timestamp with zeroes to validate sorting.
	if ev.OriginServerTime >= 0 {
		str = i64ZeroPadding[len(i64ZeroPadding)-len(str):] + str
	} else {
		// Account for negative number.
		str = "-" + i64ZeroPadding[len(i64ZeroPadding)-len(str)-2:] + str[1:]
	}

	// use \x01 to avoid colliding delimiter
	return str + "\x01" + string(ev.ID)
}

func (p *dbPaths) setTimeline(n db.Node, roomID matrix.RoomID, tl api.SyncTimeline) {
	tnode := n.FromPath(p.timelineEventsPath(roomID))

	for i := range tl.Events {
		tl.Events[i].RoomID = roomID

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

func (p *dbPaths) setDirect(n db.Node, roomID matrix.RoomID, direct bool) {
	n = n.FromPath(p.directs)

	var err error
	if direct {
		err = n.Set(string(roomID), struct{}{})
	} else {
		err = n.Delete(string(roomID))
	}

	if err != nil {
		log.Printf("failed to save direct room %q: %v", roomID, err)
	}
}
