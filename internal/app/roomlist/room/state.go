package room

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/registry"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

// StateHandlerFunc is a function called when something in the state changes.
type StateHandlerFunc func(context.Context, State)

// StateHandlers holds multiple state handlers.
type StateHandlers registry.Registry

// Add adds fn into the registry.
func (h *StateHandlers) Add(fn StateHandlerFunc) func() {
	return (*registry.Registry)(h).Add(fn, nil).Delete
}

func (h *StateHandlers) invoke(ctx context.Context, state State) {
	(*registry.Registry)(h).Each(func(v, _ interface{}) {
		fn := v.(StateHandlerFunc)
		fn(ctx, state)
	})
}

// State handles the room's internal state.
type State struct {
	ID     matrix.RoomID
	Name   string
	Topic  string
	Avatar matrix.URL

	handlers struct {
		Name   StateHandlers
		Topic  StateHandlers
		Avatar StateHandlers
	}

	ctx context.Context
}

// StateChangeFuncs contains functions that are called when the state is
// invalidated and sees a change in any of the fields.
type StateChangeFuncs struct {
	Name   func(context.Context, State)
	Avatar func(context.Context, State)
}

// NewState creates a new state.
func NewState(ctx context.Context, id matrix.RoomID) *State {
	return &State{
		ID:   id,
		Name: string(id),
		ctx:  ctx,
	}
}

// roomStateEvents is the list of room state events to subscribe to.
var roomStateEvents = []event.Type{
	event.TypeRoomName,
	event.TypeRoomCanonicalAlias,
	event.TypeRoomAvatar,
	event.TypeRoomTopic,
}

// Subscribe subscribes the State to update itself.
func (s *State) Subscribe() (unsub func()) {
	client := gotktrix.FromContext(s.ctx)
	ctx := s.ctx

	s.InvalidateName(ctx)
	s.InvalidateTopic(ctx)
	s.InvalidateAvatar(ctx)

	return client.SubscribeRoomEvents(s.ID, roomStateEvents, func(ev event.Event) {
		gtkutil.IdleCtx(ctx, func() {
			switch ev.(type) {
			case *event.RoomNameEvent, *event.RoomCanonicalAliasEvent:
				s.InvalidateName(ctx)
			case *event.RoomTopicEvent:
				s.InvalidateTopic(ctx)
			case *event.RoomAvatarEvent:
				s.InvalidateAvatar(ctx)
			}
		})
	})
}

// NotifyName calls f when Name is changed.
func (s *State) NotifyName(f StateHandlerFunc) func() {
	f(s.ctx, *s)
	return s.handlers.Name.Add(f)
}

// NotifyTopic calls f when Topic is changed.
func (s *State) NotifyTopic(f StateHandlerFunc) func() {
	f(s.ctx, *s)
	return s.handlers.Topic.Add(f)
}

// NotifyAvatar calls f when Avatar is changed.
func (s *State) NotifyAvatar(f StateHandlerFunc) func() {
	f(s.ctx, *s)
	return s.handlers.Avatar.Add(f)
}

// InvalidateName invalidates the room's name and refetches them from the state
// or API.
func (s *State) InvalidateName(ctx context.Context) {
	client := gotktrix.FromContext(ctx)

	n, err := client.Offline().RoomName(s.ID)
	if err == nil && n != "Empty Room" && n != s.Name {
		s.Name = n
		s.handlers.Name.invoke(ctx, *s)
		return
	}

	go func() {
		n, err := client.RoomName(s.ID)
		if err == nil {
			glib.IdleAdd(func() {
				if s.Name != n {
					s.Name = n
					s.handlers.Name.invoke(ctx, *s)
				}
			})
		}
	}()
}

// InvalidateTopic invalidates the room's name and refetches them from the state
// or API.
func (s *State) InvalidateTopic(ctx context.Context) {
	client := gotktrix.FromContext(s.ctx).Offline()

	e, err := client.Offline().RoomState(s.ID, event.TypeRoomTopic, "")
	if err == nil {
		var topic string
		if nameEvent, ok := e.(*event.RoomTopicEvent); ok {
			topic = nameEvent.Topic
		}

		if topic != s.Topic {
			s.Topic = topic
			s.handlers.Topic.invoke(ctx, *s)
		}
	}
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
		s.handlers.Avatar.invoke(ctx, *s)
	}
}
