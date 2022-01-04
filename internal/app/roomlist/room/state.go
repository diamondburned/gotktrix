package room

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
)

// State handles the room's internal state.
type State struct {
	ID     matrix.RoomID
	Name   string
	Avatar matrix.URL

	ctx   context.Context
	funcs StateChangeFuncs
}

// StateChangeFuncs contains functions that are called when the state is
// invalidated and sees a change in any of the fields.
type StateChangeFuncs struct {
	Name   func(context.Context, State)
	Avatar func(context.Context, State)
}

var stateEvents = []event.Type{
	event.TypeRoomName,
	event.TypeRoomCanonicalAlias,
	event.TypeRoomAvatar,
}

// NewState creates a new state.
func NewState(ctx context.Context, id matrix.RoomID, funcs StateChangeFuncs) *State {
	return &State{
		ID:    id,
		Name:  string(id),
		ctx:   ctx,
		funcs: funcs,
	}
}

// Subscribe subscribes the State to update itself.
func (s *State) Subscribe() (unsub func()) {
	client := gotktrix.FromContext(s.ctx)
	ctx := s.ctx

	s.InvalidateName(ctx)
	s.InvalidateAvatar(ctx)

	return client.SubscribeRoomEvents(s.ID, roomEvents, func(ev event.Event) {
		gtkutil.IdleCtx(ctx, func() {
			switch ev.(type) {
			case *event.RoomNameEvent, *event.RoomCanonicalAliasEvent:
				s.InvalidateName(ctx)
			case *event.RoomAvatarEvent:
				s.InvalidateAvatar(ctx)
			}
		})
	})
}

// InvalidateName invalidates the room's name and refetches them from the state
// or API.
func (s *State) InvalidateName(ctx context.Context) {
	client := gotktrix.FromContext(ctx)

	n, err := client.Offline().RoomName(s.ID)
	if err == nil {
		s.Name = n
		s.funcs.Name(ctx, *s)
		return
	}

	go func() {
		n, err := client.RoomName(s.ID)
		if err == nil {
			glib.IdleAdd(func() {
				s.Name = n
				s.funcs.Name(ctx, *s)
			})
		}
	}()
}

// InvalidateAvatar invalidates the room's avatar.
func (s *State) InvalidateAvatar(ctx context.Context) {
	client := gotktrix.FromContext(ctx)

	mxc, err := client.Offline().RoomAvatar(s.ID)
	if err == nil {
		s.setAvatar(ctx, mxc)
		return
	}

	go func() {
		mxc, _ := client.RoomAvatar(s.ID)
		glib.IdleAdd(func() { s.setAvatar(ctx, mxc) })
	}()
}

func (s *State) setAvatar(ctx context.Context, mxc *matrix.URL) {
	var url matrix.URL
	if mxc != nil {
		url = *mxc
	}

	if s.Avatar != url {
		s.Avatar = url
		s.funcs.Avatar(ctx, *s)
	}
}
