package state

import (
	"encoding/json"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/diamondburned/gotktrix/internal/gotktrix/events/sys"
	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/db"
	"github.com/diamondburned/gotrix/api"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
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

// eventBase should be kept in sync with some of EventInfo and StateEventInfo.
type eventBase struct {
	Type     event.Type `json:"type"`
	StateKey string     `json:"state_key"`
}

func setRawEvent(n db.Node, roomID matrix.RoomID, raw event.RawEvent, state bool) {
	var base eventBase
	if err := json.Unmarshal(raw, &base); err != nil {
		return // ignore
	}

	n = n.Node(string(base.Type))

	var err error
	if state {
		err = n.Set(base.StateKey, raw)
	} else {
		err = n.SetIfNone(base.StateKey, raw)
	}

	if err != nil {
		log.Printf("failed to set Matrix event for room %q: %v", roomID, err)
	}
}

func (p *dbPaths) setRaws(
	n db.Node, roomID matrix.RoomID, raws []event.RawEvent, state bool) {

	if roomID != "" {
		n = n.FromPath(p.rooms)
		n = n.Node(string(roomID))
	} else {
		n = n.FromPath(p.user)
	}

	for _, raw := range raws {
		setRawEvent(n, roomID, raw, state)
	}
}

func (p *dbPaths) setStrippeds(
	n db.Node, roomID matrix.RoomID, raws []event.StrippedEvent, state bool) {

	if roomID != "" {
		n = n.FromPath(p.rooms)
		n = n.Node(string(roomID))
	} else {
		n = n.FromPath(p.user)
	}

	for _, raw := range raws {
		setRawEvent(n, roomID, event.RawEvent(raw), state)
	}
}

func (p *dbPaths) setSummary(n db.Node, roomID matrix.RoomID, s api.SyncRoomSummary) {
	if roomID == "" {
		return // unexpecting
	}

	if err := n.FromPath(p.summaries).SetAny(string(roomID), &s); err != nil {
		log.Printf("failed to set Matrix room summary for room %q: %v", roomID, err)
	}
}

func (p *dbPaths) setRoomAny(n db.Node, roomID matrix.RoomID, k string, v interface{}) {
	if err := n.FromPath(p.rooms).Node(string(roomID)).SetAny(k, v); err != nil {
		log.Printf("failed to set room key %q: %v", k, err)
	}
}

func (p *dbPaths) timelineNode(n db.Node, roomID matrix.RoomID) db.Node {
	return n.FromPath(p.timelines).Node(string(roomID))
}

func (p *dbPaths) timelineEventsNode(n db.Node, roomID matrix.RoomID) db.Node {
	return p.timelineNode(n, roomID).Node("events")
}

var i64ZeroPadding = func() string {
	v := len(strconv.FormatInt(math.MaxInt64, 32))
	return strings.Repeat("0", v)
}()

// timelineEventBase should be kept in sync with event.RoomEvent.
type timelineEventBase struct {
	ID               matrix.EventID   `json:"event_id,omitempty"`
	OriginServerTime matrix.Timestamp `json:"origin_server_ts"`
}

// timelineEventKey formats a key that the internal database can use to sort the
// returned values lexicographically.
func timelineEventKey(ev event.RawEvent) string {
	var base timelineEventBase
	json.Unmarshal(ev, &base)

	str := strconv.FormatInt(int64(base.OriginServerTime), 32)
	// Pad the timestamp with zeroes to validate sorting.
	if base.OriginServerTime >= 0 {
		str = i64ZeroPadding[len(i64ZeroPadding)-len(str):] + str
	} else {
		// Account for negative number.
		str = "-" + i64ZeroPadding[len(i64ZeroPadding)-len(str)-2:] + str[1:]
	}

	// use \x01 to avoid colliding delimiter
	return str + "\x01" + string(base.ID)
}

func (p *dbPaths) setTimeline(n db.Node, roomID matrix.RoomID, tl api.SyncTimeline) {
	tnode := p.timelineEventsNode(n, roomID)

	for _, raw := range tl.Events {
		key := timelineEventKey(raw)
		if err := tnode.Set(key, raw); err != nil {
			log.Printf("failed to set Matrix timeline event for room %q: %v", roomID, err)
		}
	}

	// Clean up the timeline events.
	if err := tnode.DropExceptLast(TimelineKeepLast); err != nil {
		log.Printf("failed to clean up Matrix timeline for room %q: %v", roomID, err)
	}

	// Write the previous batch string, if any.
	if tl.PreviousBatch != "" {
		rnode := p.timelineNode(n, roomID)

		if err := rnode.Set("previous_batch", []byte(tl.PreviousBatch)); err != nil {
			log.Printf("failed to set previous_batch for room %q: %v", roomID, err)
		}
	}
}

func (p *dbPaths) deleteTimeline(n db.Node, roomID matrix.RoomID) {
	n = p.timelineNode(n, roomID)

	if err := n.Drop(); err != nil {
		log.Printf("failed to delete Matrix timeline for room %q: %v", roomID, err)
	}
}

func (p *dbPaths) setDirect(n db.Node, roomID matrix.RoomID, direct bool) {
	n = n.FromPath(p.directs)

	var err error
	if direct {
		err = n.Set(string(roomID), nil)
	} else {
		err = n.Delete(string(roomID))
	}

	if err != nil {
		log.Printf("failed to save direct room %q: %v", roomID, err)
	}
}

func getEvent(n db.Node, k string, expect event.Type) (event.Event, error) {
	var ev event.Event
	err := n.Get(k, eventFunc(&ev, expect))
	return ev, err
}

func eventFunc(ev *event.Event, expect event.Type) func(b []byte) error {
	return func(b []byte) (err error) {
		*ev, err = sys.ParseAs(b, expect)
		return
	}
}
