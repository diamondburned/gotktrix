package handler

import (
	"container/list"
	"context"
	"sync"

	"github.com/chanbakjsd/gotrix"
	"github.com/chanbakjsd/gotrix/api"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/state"
)

type Registry struct {
	mut sync.Mutex

	timeline map[matrix.RoomID]*list.List
	roomFns  map[matrix.RoomID]eventHandlers
	userFns  eventHandlers

	// on-sync handlers
	sync list.List

	caughtUp bool
}

// New creates a new handler registry.
func New() *Registry {
	return &Registry{
		timeline: make(map[matrix.RoomID]*list.List, 100),
		roomFns:  make(map[matrix.RoomID]eventHandlers, 100),
		userFns:  newEventHandlers(100),
	}
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

	e := r.sync.PushBack(f)
	return listRemover(&r.mut, &r.sync, e)
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

// SubscribeTimeline subscribes the given function to the timeline of a room. If
// the returned callback is called, then the room is removed from the handlers.
func (r *Registry) SubscribeTimeline(rID matrix.RoomID, f interface{}) func() {
	r.mut.Lock()
	defer r.mut.Unlock()

	tl, ok := r.timeline[rID]
	if !ok {
		tl = list.New()
		r.timeline[rID] = tl
	}

	e := tl.PushBack(f)
	return listRemover(&r.mut, tl, e)
}

// SubscribeUser subscribes the given function with the given event type to be
// called on each user event. If typ is "*", then all events are called w/ it.
func (r *Registry) SubscribeUser(typ event.Type, f interface{}) func() {
	r.mut.Lock()
	defer r.mut.Unlock()

	return r.userFns.addRm(&r.mut, typ, f)
}

// SubscribeRoom subscribes the given function to a room's state and ephemeral
// event.
func (r *Registry) SubscribeRoom(rID matrix.RoomID, typ event.Type, f interface{}) func() {
	return r.SubscribeRoomEvents(rID, []event.Type{typ}, f)
}

// SubscribeRoomStateKey is similarly to SubscribeRoom, except it only filters
// for the given state key.
func (r *Registry) SubscribeRoomStateKey(
	rID matrix.RoomID, typ event.Type, key string, f interface{}) func() {

	return r.SubscribeRoom(rID, typ, func(ivk *eventInvoker) {
		if ivk.raw.StateKey != key {
			return
		}

		ivk.invoke(f)
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
		sh = newEventHandlers(20)
		r.roomFns[rID] = sh
	}

	return sh.addEvsRm(&r.mut, types, f)
}

func listRemover(mu sync.Locker, l *list.List, e *list.Element) func() {
	return func() {
		mu.Lock()
		l.Remove(e)
		mu.Unlock()
	}
}

// AddEvents satisfies part of gotrix.State.
func (r *Registry) AddEvents(sync *api.SyncResponse) error {
	r.mut.Lock()
	defer r.mut.Unlock()

	invokeSync(&r.sync, sync)

	r.invokeUser(sync.Presence.Events)
	r.invokeUser(sync.AccountData.Events)
	r.invokeUser(sync.ToDevice.Events)

	for k, v := range sync.Rooms.Joined {
		r.invokeRoom(k, v.State.Events)
		r.invokeRoom(k, v.Ephemeral.Events)
		r.invokeRoom(k, v.AccountData.Events)
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
	for i := range raws {
		ev := eventInvoke("", &raws[i])
		r.userFns.invoke(&ev)
	}
}

func (r *Registry) invokeRoomStripped(rID matrix.RoomID, stripped []event.StrippedEvent) {
	rh, ok := r.roomFns[rID]
	if !ok {
		return
	}

	for i := range stripped {
		ev := eventInvoke(rID, &stripped[i].RawEvent)
		rh.invoke(&ev)
	}
}

func (r *Registry) invokeRoom(rID matrix.RoomID, raws []event.RawEvent) {
	rh, ok := r.roomFns[rID]
	if !ok {
		return
	}

	for i := range raws {
		ev := eventInvoke(rID, &raws[i])
		rh.invoke(&ev)
	}
}

func (r *Registry) invokeTimeline(rID matrix.RoomID, raws []event.RawEvent) {
	rh, ok := r.timeline[rID]
	if !ok {
		return
	}

	if len(raws) > state.TimelineKeepLast {
		// Only dispatch the latest 100 room events.
		raws = raws[len(raws)-state.TimelineKeepLast:]
	}

	for i := range raws {
		ev := eventInvoke(rID, &raws[i])
		ev.invokeList(rh)
	}
}
