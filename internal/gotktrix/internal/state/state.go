package state

import (
	"encoding/json"
	"log"
	"sync"

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
	TimelineKeepLast = 50
	// Version is the incremental database version number. It is incremented
	// when a breaking change is made in the database that breaks old databases.
	Version = 1
)

// State is a disk-based database of the Matrix state. Note that methods that
// get multiple events will ignore unknown events, while methods that get a
// single event will error out when that happens.
type State struct {
	db    *db.KV
	top   db.Node
	paths dbPaths

	idErr  error
	idUser matrix.UserID
	idMut  sync.RWMutex
}

// New creates a new State using bbolt pointing to the given path.
func New(path string) (*State, error) {
	kv, err := db.NewKVFile(path)
	if err != nil {
		return nil, err
	}

	return NewWithDatabase(kv)
}

// NewWithDatabase creates a new State with the given kvpack database.
func NewWithDatabase(kv *db.KV) (*State, error) {
	topPath := db.NewNodePath("gotktrix")
	topNode := kv.NodeFromPath(topPath)

	// Confirm version.
	var version int

	// Version is provided, so old database.
	if err := topNode.Get("version", &version); err == nil && version < Version {
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
		db:    kv,
		top:   kv.NodeFromPath(topPath),
		paths: newDBPaths(topPath),
	}, nil
}

// Close closes the internal database.
func (s *State) Close() error {
	return s.db.Close()
}

// Whoami returns the cached user ID. An error is returned if nothing is yet
// cached.
func (s *State) Whoami() (matrix.UserID, error) {
	s.idMut.RLock()
	defer s.idMut.RUnlock()

	if s.idUser == "" && s.idErr == nil {
		return "", errors.New("whoami not fetched")
	}

	return s.idUser, s.idErr
}

// SetWhoami sets the internal user ID.
func (s *State) SetWhoami(uID matrix.UserID) {
	s.idMut.Lock()
	s.idUser = uID
	s.idMut.Unlock()
}

// RoomState returns the last event set by RoomEventSet. It never returns an
// error as it does not forget state.
func (s *State) RoomState(roomID matrix.RoomID, typ event.Type, key string) (event.StateEvent, error) {
	raw := event.RawEvent{RoomID: roomID}

	var dbKey string
	if key != "" {
		dbKey = db.Keys(string(roomID), string(typ), key)
	} else {
		// Prevent trailing delimiter; see setRawEvent.
		dbKey = db.Keys(string(roomID), string(typ))
	}

	if err := s.db.NodeFromPath(s.paths.rooms).Get(dbKey, &raw); err != nil {
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
func (s *State) RoomStates(roomID matrix.RoomID, typ event.Type) (map[string]event.StateEvent, error) {
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
func (s *State) EachRoomState(
	roomID matrix.RoomID, typ event.Type, f func(string, event.StateEvent) error) error {

	raw := event.RawEvent{RoomID: roomID}
	path := s.paths.rooms.Tail(string(roomID), string(typ))

	return s.db.NodeFromPath(path).Each(&raw, "", func(_ string, total int) error {
		e, err := raw.Parse()
		if err != nil {
			return nil
		}

		state, ok := e.(event.StateEvent)
		if !ok {
			return nil
		}

		if err := f(raw.StateKey, state); err != nil {
			if errors.Is(err, gotrix.ErrStopIter) {
				return db.EachBreak
			}
			return err
		}
		return nil
	})
}

// EachRoomStateLen is a variant of EachRoomState, but it works for any event,
// and a length parameter is precalculated.
func (s *State) EachRoomStateLen(
	roomID matrix.RoomID, typ event.Type, f func(ev event.Event, total int) error) error {

	raw := event.RawEvent{RoomID: roomID}
	path := s.paths.rooms.Tail(string(roomID), string(typ))

	return s.db.NodeFromPath(path).Each(&raw, "", func(_ string, total int) error {
		e, err := raw.Parse()
		if err != nil {
			return nil
		}

		return f(e, total)
	})
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
		if err := n.AllKeys(&roomIDs, ""); err != nil {
			return err
		}

		if roomIDs == nil {
			return errors.New("no rooms in state")
		}

		// Ensure that we only have joined rooms.
		filtered := roomIDs[:0]

		for _, id := range roomIDs {
			memberKey := db.Keys(string(id), string(event.TypeRoomMember), string(s.idUser))
			if !n.Exists(memberKey) {
				continue
			}

			filtered = append(filtered, id)
		}

		return nil
	})
}

// RoomPreviousBatch gets the previous batch string for the given room.
func (s *State) RoomPreviousBatch(roomID matrix.RoomID) (prev string, err error) {
	return prev, s.top.FromPath(s.paths.timelinePath(roomID)).Get("previous_batch", &prev)
}

// RoomTimeline returns the latest timeline events of a room. The order of the
// returned events are always guaranteed to be latest last.
func (s *State) RoomTimeline(roomID matrix.RoomID) ([]event.RoomEvent, error) {
	var raws []event.RawEvent

	if err := s.top.FromPath(s.paths.timelineEventsPath(roomID)).All(&raws, ""); err != nil {
		log.Printf("error getting timeline for room %q: %v", roomID, err)
		return nil, err
	}

	if raws == nil {
		return nil, errors.New("empty timeline state")
	}

	events := make([]event.RoomEvent, 0, len(raws))

	for i := range raws {
		raws[i].RoomID = roomID

		e, err := raws[i].Parse()
		if err != nil {
			// Ignore unknown events.
			continue
		}

		if e, ok := e.(event.RoomEvent); ok {
			events = append(events, e)
		} else {
			log.Printf("not a room event in timeline state: %T", e)
		}
	}

	return events, nil
}

// UserEvent gets the user event from the given type.
func (s *State) UserEvent(typ event.Type) (event.Event, error) {
	var raw event.RawEvent

	if err := s.db.NodeFromPath(s.paths.user).Get(string(typ), &raw); err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			log.Printf("error getting event type %s: %v", typ, err)
		}
		return nil, errors.Wrap(err, "event not found in state")
	}

	e, err := raw.Parse()
	if err != nil {
		log.Printf("error parsing event type %s from db: %v", typ, err)
		return nil, errors.Wrap(err, "event not found in state")
	}

	return e, nil
}

// SetUserEvent updates the user event inside the state.
func (s *State) SetUserEvent(ev event.Event) error {
	b, err := json.Marshal(ev)
	if err != nil {
		return errors.Wrap(err, "failed to marshal event")
	}

	raw := event.RawEvent{
		Type:    ev.Type(),
		Content: b,
	}

	if err := s.db.NodeFromPath(s.paths.user).Set(string(raw.Type), &raw); err != nil {
		return errors.Wrap(err, "failed to update db")
	}

	return nil
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

		// Query a RoomMemberEvent second, but we need the current user's ID to
		// do so. Exit otherwise.
		u, err := s.Whoami()
		if err != nil {
			return err
		}

		key := db.Keys(string(roomID), string(event.TypeRoomMember), string(u))
		raw := event.RawEvent{
			RoomID:   roomID,
			StateKey: string(u),
		}

		if err := s.db.NodeFromPath(s.paths.rooms).Get(key, &raw); err != nil {
			return err
		}

		e, err := raw.Parse()
		if err != nil {
			return err
		}

		ev, ok := e.(event.RoomMemberEvent)
		if !ok {
			return err
		}

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
