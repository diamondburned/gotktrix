package state

import (
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
)

// State is a disk-based database of the Matrix state. Note that methods that
// get multiple events will ignore unknown events, while methods that get a
// single event will error out when that happens.
type State struct {
	db    db.KV
	top   db.Node
	paths dbPaths

	waitMu   sync.Mutex
	syncWait map[chan<- *api.SyncResponse]bool // true -> keep

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

	return NewWithDatabase(*kv), nil
}

// NewWithDatabase creates a new State with the given kvpack database.
func NewWithDatabase(kv db.KV) *State {
	topPath := db.NewNodePath("gotktrix")

	return &State{
		db:       kv,
		top:      kv.NodeFromPath(topPath),
		paths:    newDBPaths(topPath),
		syncWait: make(map[chan<- *api.SyncResponse]bool),
	}
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

	return states, s.EachRoomState(roomID, typ, func(e event.Event, total int) error {
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

	return states, s.EachRoomState(roomID, typ, func(e event.Event, total int) error {
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

// EachRoomState calls f on every raw event in the room state.
func (s *State) EachRoomState(
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

// NextBatch returns the next batch string with true if the database contains
// the next batch event. Otherwise, an empty string with false is returned.
func (s *State) NextBatch() (next string, ok bool) {
	err := s.top.Get("next_batch", &next)
	return next, err == nil
}

// AddRoomStates adds the given room state events.
func (s *State) AddRoomMessages(roomID matrix.RoomID, resp *api.RoomMessagesResponse) {
	err := s.top.TxUpdate(func(n db.Node) error {
		s.paths.setRaws(n, roomID, resp.State)
		s.paths.setTimeline(n, roomID, api.SyncTimeline{
			Events: resp.Chunk,
		})
		return nil
	})
	if err != nil {
		log.Println("AddRoomEvents error:", err)
	}
}

// AddEvent sets the room state events inside a State to be returned by State later.
func (s *State) AddEvents(sync *api.SyncResponse) error {
	err := s.top.TxUpdate(func(n db.Node) error {
		s.paths.setRaws(n, "", sync.AccountData.Events)
		s.paths.setRaws(n, "", sync.Presence.Events)
		s.paths.setRaws(n, "", sync.ToDevice.Events)

		for k, v := range sync.Rooms.Joined {
			s.paths.setRaws(n, k, v.State.Events)
			s.paths.setRaws(n, k, v.AccountData.Events)
			s.paths.setTimeline(n, k, v.Timeline)
		}

		for k, v := range sync.Rooms.Invited {
			s.paths.setStrippeds(n, k, v.State.Events)
		}

		for k, v := range sync.Rooms.Left {
			s.paths.setRaws(n, k, v.State.Events)
			s.paths.setRaws(n, k, v.AccountData.Events)
			s.paths.deleteTimeline(n, k)
		}

		if err := n.Set("next_batch", sync.NextBatch); err != nil {
			log.Println("failed to save next_batch:", err)
		}

		return nil
	})

	s.waitMu.Lock()
	defer s.waitMu.Unlock()

	for ch, keep := range s.syncWait {
		select {
		case ch <- sync:
			if !keep {
				delete(s.syncWait, ch)
			}
		default:
			// Retry later.
		}
	}

	return err
}

// WaitForNextSync will add the channel to the registry of channels to be called
// on the next sync. Once that next sync is sent into the channel, it will be
// removed.
func (s *State) WaitForNextSync(ch chan<- *api.SyncResponse) {
	s.waitMu.Lock()
	s.syncWait[ch] = false
	s.waitMu.Unlock()
}
