package space

import (
	"context"

	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

type spaceState struct {
	filter   func()
	id       matrix.RoomID
	children spaceRooms
	cancel   context.CancelFunc
}

func newSpaceState(invalidateFilter func()) spaceState {
	return spaceState{
		filter: invalidateFilter,
	}
}

// update updates the internal room children state and calls the
// invalidateFilter callback.
func (s *spaceState) update(ctx context.Context, spaceID matrix.RoomID) {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	s.id = spaceID
	s.children.reset()

	if spaceID == "" {
		s.filter()
		return
	}

	client := gotktrix.FromContext(ctx)

	// Perform an offline fetch first.
	ok := s.children.fetch(client.Offline(), spaceID)
	s.filter()

	if ok {
		// Fetched successfully, so just bail early.
		return
	}

	// Create a new cancellable context, so the next time the user rapidly spams
	// the spaces buttons, we don't try to race the state storage with a bunch
	// of junk.
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	gtkutil.Async(ctx, func() func() {
		var children spaceRooms
		children.fetch(client, spaceID)

		return func() {
			s.children = children
			s.filter()

			s.cancel()
			s.cancel = nil
		}
	})
}

// spaceRooms is a set of room IDs for the purpose of tracking which rooms are
// in a space.
type spaceRooms map[matrix.RoomID]struct{}

func (s spaceRooms) has(roomID matrix.RoomID) bool {
	_, has := s[roomID]
	return has
}

func (s *spaceRooms) reset() {
	*s = nil
}

// fetch populates spaceRooms with all room IDs inside a certain given space.
func (s *spaceRooms) fetch(client *gotktrix.Client, spaceID matrix.RoomID) bool {
	// It's fine if we use the online context here, since the events that we
	// receive from the API will be saved into the state for the next time.
	err1 := client.EachRoomStateLen(spaceID, m.SpaceChildEventType,
		func(ev event.StateEvent, len int) error {
			if *s == nil {
				*s = make(map[matrix.RoomID]struct{}, len)
			}

			space := ev.(*m.SpaceChildEvent)
			(*s)[space.ChildRoomID()] = struct{}{}
			return nil
		},
	)

	// Succumb to Matrix's terrible design: iterate over each room and determine
	// if it's in our space or not. We can do this lazily, but we prefer not to.
	roomIDs, err2 := client.Rooms()
	if err2 != nil {
		return false
	}

	// At this point, *s shouldn't have been nil, but it might be if we can't
	// find the space parent in the state because the server splurged out.
	if *s == nil {
		*s = make(map[matrix.RoomID]struct{})
	}

	for _, roomID := range roomIDs {
		_, ok := (*s)[roomID]
		if ok {
			continue
		}

		// Hitting the API is super expensive and slow here, especially when the
		// rooms aren't in a space, so we hit the state only.
		e, _ := client.State.RoomState(roomID, m.SpaceParentEventType, string(spaceID))
		if e != nil {
			room := e.(*m.SpaceParentEvent)
			(*s)[room.SpaceRoomID()] = struct{}{}
		}
	}

	return err1 == nil && err2 == nil
}
