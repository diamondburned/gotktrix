package handler

import (
	"context"
	"sync"

	"github.com/diamondburned/gotktrix/internal/gotktrix/events/sys"
	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/state"
	"github.com/diamondburned/gotktrix/internal/registry"
	"github.com/diamondburned/gotrix"
	"github.com/diamondburned/gotrix/api"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

type Registry struct {
	mut sync.Mutex

	timeline map[matrix.RoomID]registry.Registry
	roomFns  map[matrix.RoomID]eventHandlers
	userFns  eventHandlers

	// on-sync handlers
	sync registry.Registry

	caughtUp bool
}

// New creates a new handler registry.
func New() *Registry {
	r := &Registry{}
	r.sync = registry.New(10)
	r.userFns = newEventHandlers(&r.mut, 100)
	r.roomFns = make(map[matrix.RoomID]eventHandlers, 100)
	r.timeline = make(map[matrix.RoomID]registry.Registry, 100)
	return r
}

// Wrap returns a state wrapper that wraps the existing gotrix.State to also
// call Registry.
func (r *Registry) Wrap(state gotrix.State) gotrix.State {
	return wrapper{state, r}
}

// OnSync is called after the state updates on every sync.
func (r *Registry) OnSync(f func(*api.SyncResponse)) func() {
	r.mut.Lock()
	defer r.mut.Unlock()

	return valueRemover(&r.mut, r.sync.Add(f, nil))
}

// OnSyncCh sends into the channel every sync until the returned callback is
// called.
func (r *Registry) OnSyncCh(ctx context.Context, ch chan<- *api.SyncResponse) {
	incomingSync := make(chan *api.SyncResponse)
	rm := r.OnSync(func(sync *api.SyncResponse) { incomingSync <- sync })

	go func() {
		var sync *api.SyncResponse
		var send chan<- *api.SyncResponse

		for {
			select {
			case sync = <-incomingSync:
				send = ch
			case send <- sync:
				// ok
			case <-ctx.Done():
				rm()
				return
			}
		}
	}()
}

// SubscribeAllTimeline subscribes to the timeline of all rooms.
func (r *Registry) SubscribeAllTimeline(f interface{}) func() {
	return r.subscribeTimeline("*", f, handlerMeta{})
}

// SubscribeTimeline subscribes the given function to the timeline of a room. If
// the returned callback is called, then the room is removed from the handlers.
func (r *Registry) SubscribeTimeline(rID matrix.RoomID, f interface{}) func() {
	return r.subscribeTimeline(rID, f, handlerMeta{})
}

// SubscribeTimelineSync is similar to SubscribeTimeline, except f is only
// called on the latest event each sync instead of on all of them.
func (r *Registry) SubscribeTimelineSync(rID matrix.RoomID, f interface{}) func() {
	return r.subscribeTimeline(rID, f, handlerMeta{once: true})
}

func (r *Registry) subscribeTimeline(rID matrix.RoomID, f interface{}, meta handlerMeta) func() {
	r.mut.Lock()
	defer r.mut.Unlock()

	tl, ok := r.timeline[rID]
	if !ok {
		tl = registry.New(2)
		r.timeline[rID] = tl
	}

	return valueRemover(&r.mut, tl.Add(f, meta))
}

func valueRemover(mu *sync.Mutex, v *registry.Value) func() {
	return func() {
		// Workaround in some cases where the callback triggers a removal that
		// acquires the same mutex.
		go func() {
			mu.Lock()
			v.Delete()
			mu.Unlock()
		}()
	}
}

// SubscribeUser subscribes the given function with the given event type to be
// called on each user event. If typ is "*", then all events are called w/ it.
func (r *Registry) SubscribeUser(typ event.Type, f interface{}) func() {
	r.mut.Lock()
	defer r.mut.Unlock()

	return r.userFns.addRm(typ, f, handlerMeta{})
}

// SubscribeRoom subscribes the given function to a room's state and ephemeral
// event.
func (r *Registry) SubscribeRoom(rID matrix.RoomID, typ event.Type, f interface{}) func() {
	return r.SubscribeRoomEvents(rID, []event.Type{typ}, f)
}

type roomSyncEvent struct{}

const roomSyncEventType event.Type = "__roomSyncEvent"

func (ev roomSyncEvent) Info() *event.EventInfo {
	return &event.EventInfo{Type: roomSyncEventType}
}

// SubscribeRoomSync subscribes f to be called every time the room is synced.
func (r *Registry) SubscribeRoomSync(rID matrix.RoomID, f func()) func() {
	return r.SubscribeRoom(rID, roomSyncEventType, f)
}

// SubscribeRoomStateKey is similarly to SubscribeRoom, except it only filters
// for the given state key.
func (r *Registry) SubscribeRoomStateKey(
	rID matrix.RoomID, typ event.Type, key string, f interface{}) func() {

	return r.SubscribeRoom(rID, typ, func(ev event.StateEvent) {
		if ev.StateInfo().StateKey != key {
			return
		}

		invoke(ev, f)
	})
}

// SubscribeRoomEvents is like SubscribeRoom but registers multiple events at
// once.
func (r *Registry) SubscribeRoomEvents(
	rID matrix.RoomID, types []event.Type, f interface{}) func() {

	r.mut.Lock()
	defer r.mut.Unlock()

	sh, ok := r.roomFns[rID]
	if !ok {
		sh = newEventHandlers(&r.mut, 20)
		r.roomFns[rID] = sh
	}

	return sh.addEvsRm(types, f, handlerMeta{})
}

// AddEvents satisfies part of gotrix.State.
func (r *Registry) AddEvents(sync *api.SyncResponse) error {
	r.mut.Lock()
	defer r.mut.Unlock()

	invokeSync(r.sync, sync)

	r.invokeUser(sync.Presence.Events)
	r.invokeUser(sync.AccountData.Events)
	r.invokeUser(sync.ToDevice.Events)

	for k, v := range sync.Rooms.Joined {
		r.invokeRoom(k, v.State.Events)
		r.invokeRoom(k, v.Ephemeral.Events)
		r.invokeRoom(k, v.AccountData.Events)
		r.invokeRoomSingle(k, roomSyncEvent{})
	}

	for k, v := range sync.Rooms.Invited {
		r.invokeRoomStripped(k, v.State.Events)
	}

	for k, v := range sync.Rooms.Left {
		r.invokeRoom(k, v.State.Events)
		r.invokeRoom(k, v.AccountData.Events)
	}

	if r.caughtUp {
		for k, v := range sync.Rooms.Joined {
			r.invokeTimeline(k, v.Timeline.Events)
		}
		for k, v := range sync.Rooms.Left {
			r.invokeTimeline(k, v.Timeline.Events)
		}
	} else {
		r.caughtUp = true
	}

	return nil
}

func (r *Registry) invokeUser(raws []event.RawEvent) {
	for _, ev := range sys.ParseAll(raws) {
		r.userFns.invoke(ev)
	}
}

func (r *Registry) invokeRoomStripped(rID matrix.RoomID, stripped []event.StrippedEvent) {
	for _, id := range []matrix.RoomID{rID, "*"} {
		rh, ok := r.roomFns[id]
		if !ok {
			continue
		}

		for _, raw := range stripped {
			invokeHandlers(sys.ParseRoom(raw, rID), rh)
		}
	}
}

func (r *Registry) invokeRoom(rID matrix.RoomID, raws []event.RawEvent) {
	for _, id := range []matrix.RoomID{rID, "*"} {
		rh, ok := r.roomFns[id]
		if !ok {
			continue
		}

		for _, ev := range sys.ParseAllRoom(raws, rID) {
			invokeHandlers(ev, rh)
		}
	}
}

func (r *Registry) invokeRoomSingle(rID matrix.RoomID, ev event.Event) {
	for _, id := range []matrix.RoomID{rID, "*"} {
		rh, ok := r.roomFns[id]
		if !ok {
			continue
		}

		invokeHandlers(ev, rh)
	}
}

func (r *Registry) invokeTimeline(rID matrix.RoomID, raws []event.RawEvent) {
	if len(raws) == 0 {
		return
	}

	if len(raws) > state.TimelineKeepLast {
		// Only dispatch the latest 100 room events.
		raws = raws[len(raws)-state.TimelineKeepLast:]
	}

	for _, id := range []matrix.RoomID{rID, "*"} {
		if rh, ok := r.timeline[id]; ok {
			r.invokeTimelineRegistry(rID, rh, raws)
		}
	}
}

func (r *Registry) invokeTimelineRegistry(
	rID matrix.RoomID, rh registry.Registry, raws []event.RawEvent) {

	if rh.IsEmpty() {
		return
	}

	var evs []event.Event
	var once event.Event

	rh.Each(func(f, meta interface{}) {
		hmeta, _ := meta.(handlerMeta)
		if !hmeta.once {
			if evs == nil {
				evs = sys.ParseAllRoom(raws, rID)
			}

			for _, ev := range evs {
				invokeList(ev, rh)
			}

			return
		}

		if once == nil {
			if evs != nil {
				once = evs[len(evs)-1]
			} else {
				once = sys.ParseRoom(raws[len(raws)-1], rID)
			}
		}

		invokeList(once, rh)
	})
}
