package state

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/chanbakjsd/gotrix"
	"github.com/chanbakjsd/gotrix/api"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/db"
	"github.com/pkg/errors"
)

const (
	// TimelineKeepLast determines that, when it's time to clean up, the
	// database should only keep the last 50 events.
	TimelineKeepLast = 100
	// Version is the incremental database version number. It is incremented
	// when a breaking change is made in the database that breaks old databases.
	Version = 4
)

// State is a disk-based database of the Matrix state. Note that methods that
// get multiple events will ignore unknown events, while methods that get a
// single event will error out when that happens.
type State struct {
	db     *db.KV
	top    db.Node
	paths  dbPaths
	userID matrix.UserID

	// caches
	memberNames memberNameCache
}

// New creates a new State using bbolt pointing to the given path.
func New(path string, userID matrix.UserID) (*State, error) {
	kv, err := db.NewKVFile(path)
	if err != nil {
		return nil, err
	}

	topPath := db.NewNodePath("gotktrix")
	topNode := kv.NodeFromPath(topPath)

	// Confirm version.
	var version int

	// Version is provided, so old database.
	if err := topNode.Get("version", &version); err == nil && version != Version {
		// Database is too outdated; wipe it.
		if err := kv.DropPrefix(topPath); err != nil {
			return nil, errors.Wrap(err, "failed to wipe old state")
		}
	}

	// Write the new version.
	if err := topNode.Set("version", Version); err != nil {
		return nil, errors.Wrap(err, "failed to write version")
	}

	return &State{
		db:     kv,
		top:    kv.NodeFromPath(topPath),
		paths:  newDBPaths(topPath),
		userID: userID,
	}, nil
}

// Close closes the internal database.
func (s *State) Close() error {
	return s.db.Close()
}

// RoomIsSet gets an arbitrary boolean assigned to each room using the given
// key. It's mostly used for lazy fetching in gotktrix.
func (s *State) RoomIsSet(roomID matrix.RoomID, key string) bool {
	var v bool
	s.top.FromPath(s.paths.rooms.Tail(string(roomID), "_roombool")).Get(key, &v)
	return v
}

// SetRoom sets the room's boolean key, unless if the key is already set before,
// then false is returned, otherwise true.
//
// Atomically, it can be described as an atomic compare-and-swap instruction:
//
//    atomic.CompareAndSwapUint32(&v, 0, 1) -> set:bool
//
func (s *State) SetRoom(roomID matrix.RoomID, key string) (set bool) {
	// Premature check using a RO transaction.
	if s.RoomIsSet(roomID, key) {
		return false
	}

	n := s.top.FromPath(s.paths.rooms.Tail(string(roomID), "_roombool"))
	n.TxUpdate(func(n db.Node) error {
		// Double-check the database to avoid racing other callers.
		if err := n.Get(key, &set); err == nil {
			// set == true, so we exit.
			set = false
			return err
		}

		set = true
		return n.Set(key, set)
	})

	return
}

// ResetRoom resets the room's boolean key. After this call, RoomIsSet will
// return false.
func (s *State) ResetRoom(roomID matrix.RoomID, key string) {
	s.top.FromPath(s.paths.rooms.Tail(string(roomID), "_roombool")).Delete(key)
}

// RoomEvent queries the event with the given type. If the event type implies a
// state event, then the empty key is tried.
func (s *State) RoomEvent(roomID matrix.RoomID, typ event.Type) (event.Event, error) {
	raw := event.RawEvent{
		Type:   typ,
		RoomID: roomID,
	}

	// Prevent trailing delimiter; see setRawEvent.
	n := s.db.NodeFromPath(s.paths.rooms).Node(string(roomID), string(typ))

	if err := n.Get("", &raw); err != nil {
		return nil, err
	}

	return raw.Parse()
}

// RoomState returns the last event set by RoomEventSet. It never returns an
// error as it does not forget state.
func (s *State) RoomState(
	roomID matrix.RoomID, typ event.Type, key string) (event.StateEvent, error) {

	n := s.db.NodeFromPath(s.paths.rooms).Node(string(roomID), string(typ))

	raw := event.RawEvent{
		Type:     typ,
		RoomID:   roomID,
		StateKey: key,
	}

	if err := n.Get(key, &raw); err != nil {
		return nil, err
	}

	e, err := raw.Parse()
	if err != nil {
		return nil, err
	}

	state, ok := e.(event.StateEvent)
	if !ok {
		return nil, gotrix.ErrInvalidStateEvent
	}

	return state, nil
}

// RoomStates returns the last set of events set by RoomEventSet.
func (s *State) RoomStates(
	roomID matrix.RoomID, typ event.Type) (map[string]event.StateEvent, error) {

	var states map[string]event.StateEvent

	return states, s.EachRoomStateLen(roomID, typ, func(e event.Event, total int) error {
		state, ok := e.(event.StateEvent)
		if ok {
			if states == nil {
				states = make(map[string]event.StateEvent, total)
			}
			states[state.StateKey()] = state
		}

		return nil
	})
}

// RoomStateList is the equivalent of RoomStates, except a slice is returned.
func (s *State) RoomStateList(roomID matrix.RoomID, typ event.Type) ([]event.StateEvent, error) {
	var states []event.StateEvent

	return states, s.EachRoomStateLen(roomID, typ, func(e event.Event, total int) error {
		state, ok := e.(event.StateEvent)
		if ok {
			if states == nil {
				states = make([]event.StateEvent, 0, total)
			}
			states = append(states, state)
		}

		return nil
	})
}

// EachRoomState calls f on every raw event in the room state. It satisfies the
// EachRoomState method requirement inside gotrix.State, but most callers should
// not use this method, since there is no length information.
//
// Deprecated: Use EachRoomStateLen.
func (s *State) EachRoomState(
	roomID matrix.RoomID, typ event.Type, f func(string, event.StateEvent) error) error {

	return s.EachRoomStateRaw(roomID, typ, func(raw *event.RawEvent, _ int) error {
		e, err := raw.Parse()
		if err != nil {
			return nil
		}

		state, ok := e.(event.StateEvent)
		if !ok {
			return nil
		}

		return f(state.StateKey(), state)
	})
}

// EachRoomStateLen is a variant of EachRoomState, but it works for any event,
// and a length parameter is precalculated.
func (s *State) EachRoomStateLen(
	roomID matrix.RoomID, typ event.Type, f func(ev event.Event, total int) error) error {

	return s.EachRoomStateRaw(roomID, typ, func(raw *event.RawEvent, total int) error {
		e, err := raw.Parse()
		if err != nil {
			return nil
		}

		return f(e, total)
	})
}

// EachRoomStateRaw is a variant of EachRoomState where the callback gets a raw
// event instead of the parsed event.
func (s *State) EachRoomStateRaw(
	roomID matrix.RoomID, typ event.Type, f func(raw *event.RawEvent, total int) error) error {

	var raw event.RawEvent
	path := s.paths.rooms.Tail(string(roomID), string(typ))

	return s.db.NodeFromPath(path).Each(&raw, func(_ string, total int) error {
		raw.RoomID = roomID

		if err := f(&raw, total); err != nil {
			if errors.Is(err, gotrix.ErrStopIter) {
				return db.EachBreak
			}
			return err
		}

		return nil
	})
}

// memberNameCache assists in collision checking when rendering the member name.
// Since conventionally, collision checking involves looking up a room's
// members, this can get quite costly in a loop. The goal of this cache is to
// allow constant time lookup from an in-memory cache.
//
// The cache may expire after a short amount of time; its goal is to only speed
// up bulk loading of many events for speeding up loading a single room.
type memberNameCache struct {
	sync.Map // matrix.RoomID -> roomMemberCache
}

// memberNameCacheShortAge is 15s. A short cache age allows us to not worry too
// much about cache invalidation on events, since the caching time is often
// short enough to be subtle.
const memberNameCacheShortAge = 15 * time.Second

// memberNameCacheLongAge determines the duration that the whole cache will be
// removed. It's mostly for optimization.
const memberNameCacheLongAge = 2 * time.Hour

func (c *memberNameCache) gc() {
	now := time.Now()

	c.Range(func(k, v interface{}) (ok bool) {
		cache := v.(*roomMemberCache)
		cache.mu.Lock()
		defer cache.mu.Unlock()

		switch {
		case cache.when.Add(memberNameCacheLongAge).Before(now):
			// Super expired.
			c.Delete(k)
		case cache.when.Add(memberNameCacheShortAge).Before(now):
			// Expired. Wipe the map but don't force it to reallocate. This will
			// retain the map on memory, but that's alright.
			for k := range cache.names {
				delete(cache.names, k)
			}
		}

		return true
	})
}

type roomMemberCache struct {
	names map[string][]matrix.UserID
	when  time.Time
	mu    sync.Mutex
}

// RoomMembersFromName looks up the internal cache for all members inside the
// given room with the given name. It is used for MemberNames collision
// checking.
func (s *State) RoomMembersFromName(roomID matrix.RoomID, name string) []matrix.UserID {
	// Always GC before continuing. We don't want to reuse an outdated cache
	// even once.
	s.memberNames.gc()

	iv, _ := s.memberNames.LoadOrStore(roomID, &roomMemberCache{})
	cache := iv.(*roomMemberCache)
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.names == nil {
		cache.when = time.Now()

		var m struct {
			Name string `json:"displayname"`
		}

		onMember := func(raw *event.RawEvent, total int) error {
			if cache.names == nil {
				// Overallocate to avoid some reallocations down the line. This
				// has the side effect of forcing a refetch if a room is empty,
				// but that rarely ever happens.
				cache.names = make(map[string][]matrix.UserID, total)
			}

			if err := json.Unmarshal(raw.Content, &m); err != nil {
				// Skip the error.
				return nil
			}

			userID := matrix.UserID(raw.StateKey)

			if m.Name == "" {
				// Try the username instead.
				username, _, _ := userID.Parse()
				m.Name = username
			}

			cache.names[m.Name] = append(cache.names[m.Name], userID)
			return nil
		}

		s.EachRoomStateRaw(roomID, event.TypeRoomMember, onMember)
	}

	return cache.names[name]
}

// RoomSummary returns the SyncRoomSummary if a room if it's in the state.
func (s *State) RoomSummary(roomID matrix.RoomID) (api.SyncRoomSummary, error) {
	var summary api.SyncRoomSummary
	return summary, s.db.NodeFromPath(s.paths.summaries).Get(string(roomID), &summary)
}

// Rooms returns the keys of all room states in the state.
func (s *State) Rooms() ([]matrix.RoomID, error) {
	var roomIDs []matrix.RoomID

	return roomIDs, s.top.FromPath(s.paths.rooms).TxView(func(n db.Node) error {
		if err := n.EachKey(func(k string, l int) error {
			if roomIDs == nil {
				roomIDs = make([]matrix.RoomID, 0, l)
			}
			roomIDs = append(roomIDs, matrix.RoomID(k))
			return nil
		}); err != nil {
			return err
		}

		if roomIDs == nil {
			return errors.New("no rooms in state")
		}

		// Ensure that we only have joined rooms.
		filtered := roomIDs[:0]

		for _, id := range roomIDs {
			// TODO: bring this out of the loop, so it can reuse the backing
			// array. Not very important.
			if !n.Node(string(id), string(event.TypeRoomMember)).Exists(string(s.userID)) {
				continue
			}

			filtered = append(filtered, id)
		}

		roomIDs = filtered
		return nil
	})
}

// RoomPreviousBatch gets the previous batch string for the given room.
func (s *State) RoomPreviousBatch(roomID matrix.RoomID) (prev string, err error) {
	n := s.paths.timelineNode(s.top, roomID)

	if err := n.Get("previous_batch", &prev); err != nil {
		return "", err
	}

	return prev, nil
}

// RoomTimelineRaw returns the latest raw timeline events of a room. The order
// of the returned events are always guaranteed to be latest last.
func (s *State) RoomTimelineRaw(roomID matrix.RoomID) ([]event.RawEvent, error) {
	var raws []event.RawEvent
	var raw event.RawEvent

	n := s.paths.timelineEventsNode(s.top, roomID)

	if err := n.Each(&raw, func(k string, l int) error {
		if raws == nil {
			raws = make([]event.RawEvent, 0, l)
		}
		raws = append(raws, raw)
		return nil
	}); err != nil {
		log.Printf("error getting timeline for room %q: %v", roomID, err)
		return nil, err
	}

	if raws == nil {
		return nil, errors.New("empty timeline state")
	}

	return raws, nil
}

// EachTimeline iterates through the timeline.
func (s *State) EachTimeline(roomID matrix.RoomID, f func(*event.RawEvent) error) error {
	n := s.paths.timelineEventsNode(s.top, roomID)
	var raw event.RawEvent

	return n.Each(&raw, func(string, int) error {
		cpy := raw // copy raw.
		return f(&cpy)
	})
}

// EachTimelineReverse iterates through the timeline in reverse.
func (s *State) EachTimelineReverse(roomID matrix.RoomID, f func(*event.RawEvent) error) error {
	n := s.paths.timelineEventsNode(s.top, roomID)
	var raw event.RawEvent

	return n.EachReverse(&raw, func(string, int) error {
		cpy := raw // copy raw.
		return f(&cpy)
	})
}

// LatestInTimeline returns the latest event in the given room that has the
// given type. If the type is an empty string, then the latest event with any
// type is returned.
func (s *State) LatestInTimeline(roomID matrix.RoomID, t event.Type) *event.RawEvent {
	var found *event.RawEvent

	if t == "" {
		s.EachTimelineReverse(roomID, func(ev *event.RawEvent) error {
			found = ev
			return db.EachBreak
		})
		return found
	}

	n := s.paths.timelineEventsNode(s.top, roomID)
	var rawType struct {
		Type event.Type `json:"type"`
	}

	n.TxView(func(n db.Node) error {
		return n.EachReverse(&rawType, func(k string, _ int) error {
			if rawType.Type != t {
				return nil
			}

			var raw event.RawEvent
			if err := n.Get(k, &raw); err != nil {
				return err
			}

			found = &raw
			return db.EachBreak
		})
	})

	return found
}

// UserEvent gets the user event from the given type.
func (s *State) UserEvent(typ event.Type) (event.Event, error) {
	var raw event.RawEvent

	// See setRawEvent: an event is a bucket with an empty key (no state key).
	if err := s.db.NodeFromPath(s.paths.user).Node(string(typ)).Get("", &raw); err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			log.Printf("error getting event type %s: %v", typ, err)
		}
		return nil, errors.Wrap(err, "event not found in state")
	}

	e, err := raw.Parse()
	if err != nil {
		log.Printf("error parsing event type %s from db: %v", typ, err)
		return nil, errors.Wrap(err, "parse error")
	}

	return e, nil
}

// SetUserEvent updates the user event inside the state. Error checking is not
// needed, because this function shouldn't be relied on.
func (s *State) SetUserEvent(ev event.Event) {
	b, err := json.Marshal(ev)
	if err != nil {
		log.Println("failed to marshal UserEvent for setting from API:", err)
		return
	}

	raw := event.RawEvent{
		Type:    ev.Type(),
		Content: b,
	}

	// Update local state.
	setRawEvent(s.db.NodeFromPath(s.paths.user), "", &raw, false)
}

// NextBatch returns the next batch string with true if the database contains
// the next batch event. Otherwise, an empty string with false is returned.
func (s *State) NextBatch() (next string, ok bool) {
	err := s.top.Get("next_batch", &next)
	return next, err == nil
}

// AddRoomStates adds the given room state events. Note that values set here
// will never override values from /sync.
func (s *State) AddRoomMessages(roomID matrix.RoomID, resp *api.RoomMessagesResponse) {
	err := s.top.TxUpdate(func(n db.Node) error {
		s.paths.setRaws(n, roomID, resp.State, false)
		s.paths.setTimeline(n, roomID, api.SyncTimeline{
			Events: resp.Chunk,
		})
		return nil
	})
	if err != nil {
		log.Println("AddRoomEvents error:", err)
	}
}

// AddRoomEvents adds the given list of raw events. Note that values set here
// will never override values from /sync.
func (s *State) AddRoomEvents(roomID matrix.RoomID, evs []event.RawEvent) {
	s.paths.setRaws(s.top, roomID, evs, false)
}

// UseDirectEvent fills the state cache with information from the direct event.
func (s *State) UseDirectEvent(ev event.DirectEvent) {
	err := s.top.TxUpdate(func(n db.Node) error {
		for _, roomIDs := range ev {
			for _, roomID := range roomIDs {
				s.paths.setDirect(n, roomID, true)
			}
		}
		return nil
	})
	if err != nil {
		log.Println("UseDirectEvent error:", err)
	}
}

// IsDirect returns whether or not the given room is a direct messaging room. If
// no such information exists in the state, then ok=false is returned.
func (s *State) IsDirect(roomID matrix.RoomID) (is, ok bool) {
	err := s.top.TxView(func(n db.Node) error {
		// Query the m.direct event first, which is set using paths.directs.
		if n.FromPath(s.paths.directs).Exists(string(roomID)) {
			is = true
			return nil
		}

		n = n.FromPath(s.paths.rooms)
		n = n.Node(string(roomID), string(event.TypeRoomMember))

		raw := event.RawEvent{
			RoomID:   roomID,
			StateKey: string(s.userID),
		}

		if err := n.Get(string(s.userID), &raw); err != nil {
			return err
		}

		e, err := raw.Parse()
		if err != nil {
			return err
		}

		ev := e.(event.RoomMemberEvent)
		ok = ev.IsDirect

		return nil
	})

	return is, err == nil
}

// AddEvent sets the room state events inside a State to be returned by State later.
func (s *State) AddEvents(sync *api.SyncResponse) error {
	return s.top.TxUpdate(func(n db.Node) error {
		s.paths.setRaws(n, "", sync.AccountData.Events, true)
		s.paths.setRaws(n, "", sync.Presence.Events, true)
		s.paths.setRaws(n, "", sync.ToDevice.Events, true)

		for _, ev := range sync.AccountData.Events {
			if ev.Type == event.TypeDirect {
				// Cache direct events.
				e, err := ev.Parse()
				if err != nil {
					continue
				}

				for _, roomIDs := range e.(event.DirectEvent) {
					for _, roomID := range roomIDs {
						s.paths.setDirect(n, roomID, true)
					}
				}
			}
		}

		for k, v := range sync.Rooms.Joined {
			s.paths.setRaws(n, k, v.State.Events, true)
			s.paths.setRaws(n, k, v.AccountData.Events, true)
			s.paths.setSummary(n, k, v.Summary)
			s.paths.setTimeline(n, k, v.Timeline)
		}

		for k, v := range sync.Rooms.Invited {
			s.paths.setStrippeds(n, k, v.State.Events, true)
		}

		for k, v := range sync.Rooms.Left {
			s.paths.setRaws(n, k, v.State.Events, true)
			s.paths.setRaws(n, k, v.AccountData.Events, true)
			s.paths.deleteTimeline(n, k)
		}

		if err := n.Set("next_batch", sync.NextBatch); err != nil {
			log.Println("failed to save next_batch:", err)
		}

		return nil
	})
}
