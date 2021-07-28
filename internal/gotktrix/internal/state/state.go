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

// State is the default used implementation of state by the gotrix package.
type State struct {
	parent db.KV
	top    db.Node
	user   db.Node
	rooms  db.Node

	waitMu   sync.Mutex
	syncWait map[chan<- *api.SyncResponse]bool // true -> keep
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
func NewWithDatabase(db db.KV) *State {
	return &State{
		parent:   db,
		top:      db.Node("gotktrix"),
		user:     db.Node("gotktrix").Node("user"),
		rooms:    db.Node("gotktrix").Node("rooms"),
		syncWait: make(map[chan<- *api.SyncResponse]bool),
	}
}

// Close closes the internal database.
func (s *State) Close() error {
	return s.parent.Close()
}

// RoomState returns the last event set by RoomEventSet.
// It never returns an error as it does not forget state.
func (s *State) RoomState(roomID matrix.RoomID, eventType event.Type, key string) (event.StateEvent, error) {
	var raw event.RawEvent
	dbKey := db.Keys(string(roomID), string(eventType), key)

	if err := s.rooms.Get(dbKey, &raw); err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			err = nil
		}
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
func (s *State) RoomStates(roomID matrix.RoomID, eventType event.Type) (map[string]event.StateEvent, error) {
	var raw event.RawEvent
	states := make(map[string]event.StateEvent)

	dbKey := db.Keys(string(roomID), string(eventType))

	return states, s.rooms.Each(&raw, dbKey, func(k string) error {
		e, err := raw.Parse()
		if err == nil {
			state, ok := e.(event.StateEvent)
			if ok {
				states[raw.StateKey] = state
			}
		}

		return nil
	})
}

// Event gets the event from the given type.
func (s *State) Event(typ event.Type) (event.Event, error) {
	var raw event.RawEvent

	if err := s.user.Get(string(typ), &raw); err != nil {
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

func (s *State) setRaw(b db.Batcher, roomID matrix.RoomID, raw *event.RawEvent) {
	var node db.Node
	var dbKey string

	if roomID != "" {
		node = s.rooms
		dbKey = db.Keys(string(roomID), string(raw.Type), string(raw.StateKey))
	} else {
		node = s.user
		dbKey = string(raw.Type)
	}

	if err := b.WithNode(node).Set(dbKey, raw); err != nil {
		log.Println("failed to set Matrix event into db:", err)
	}
}

func (s *State) setRaws(b db.Batcher, roomID matrix.RoomID, raws []event.RawEvent) {
	for i := range raws {
		s.setRaw(b, roomID, &raws[i])
	}
}

func (s *State) setStrippeds(b db.Batcher, roomID matrix.RoomID, raws []event.StrippedEvent) {
	for i := range raws {
		s.setRaw(b, roomID, &raws[i].RawEvent)
	}
}

// AddEvent sets the room state events inside a State to be returned by State later.
func (s *State) AddEvents(sync *api.SyncResponse) error {
	err := s.top.SetBatch(func(b db.Batcher) error {
		s.setRaws(b, "", sync.AccountData.Events)
		s.setRaws(b, "", sync.Presence.Events)
		s.setRaws(b, "", sync.ToDevice.Events)

		for k, v := range sync.Rooms.Joined {
			s.setRaws(b, k, v.State.Events)
			s.setRaws(b, k, v.AccountData.Events)
		}

		for k, v := range sync.Rooms.Invited {
			s.setStrippeds(b, k, v.State.Events)
		}

		for k, v := range sync.Rooms.Left {
			s.setRaws(b, k, v.State.Events)
			s.setRaws(b, k, v.AccountData.Events)
		}

		if err := b.Set("next_batch", sync.NextBatch); err != nil {
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
