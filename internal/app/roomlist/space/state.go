package space

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
)

type spaceState struct {
	id       matrix.RoomID
	children map[matrix.RoomID]struct{}

	cancel context.CancelFunc
}

func (s *spaceState) update(ctx context.Context, start func() func()) {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	if s.id == "" {
		return
	}

	s.children = nil

	client := gotktrix.FromContext(ctx)

	err := client.State.EachRoomStateLen(s.id, m.SpaceChildEventType, s.eachEvent)
	if err == nil {
		return
	}

	// Create a new cancellable context, so the next time the user rapidly spams
	// the spaces buttons, we don't try to race the state storage with a bunch
	// of junk.
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	cpy := *s
	stop := start()

	gtkutil.Async(ctx, func() func() {
		// It's fine if we use the root context, since the events that we
		// receive from the API will be saved into the state for the next time.
		client.EachRoomStateLen(s.id, m.SpaceChildEventType, cpy.eachEvent)

		return func() {
			s.children = cpy.children
			stop()
		}
	})
}

func (s *spaceState) eachEvent(ev event.StateEvent, len int) error {
	if s.children == nil {
		s.children = make(map[matrix.RoomID]struct{}, len)
	}
	s.children[matrix.RoomID(ev.StateInfo().StateKey)] = struct{}{}
	return nil
}

func (s *spaceState) has(roomID matrix.RoomID) bool {
	if s.id == "" {
		return true
	}

	_, has := s.children[roomID]
	return has
}
