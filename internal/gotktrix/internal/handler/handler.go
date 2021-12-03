package handler

import (
	"log"
	"sync"

	"github.com/chanbakjsd/gotrix"
	"github.com/chanbakjsd/gotrix/api"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/registry"
)

type wrapper struct {
	gotrix.State
	h *Registry
}

func (w wrapper) AddEvents(sync *api.SyncResponse) error {
	err1 := w.State.AddEvents(sync)
	err2 := w.h.AddEvents(sync)
	if err1 != nil {
		return err1
	}
	return err2
}

type eventHandlers map[event.Type]registry.M

func newEventHandlers(cap int) eventHandlers {
	return make(eventHandlers, cap)
}

func (h eventHandlers) invoke(ivk *eventInvoker) {
	ivk.invokeList(h[ivk.raw.Type])
	ivk.invokeList(h["*"])
}

func (h eventHandlers) addEvsRm(l sync.Locker, types []event.Type, fn interface{}) func() {
	if len(types) == 1 {
		return h.addRm(l, types[0], fn)
	}

	elems := make([]*registry.Value, len(types))
	for i, typ := range types {
		elems[i] = h.add(typ, fn)
	}

	return func() {
		l.Lock()
		defer l.Unlock()

		for _, elem := range elems {
			elem.Delete()
		}
	}
}

func (h eventHandlers) add(typ event.Type, fn interface{}) *registry.Value {
	ls, ok := h[typ]
	if !ok {
		ls = make(registry.M)
		h[typ] = ls
	}

	return ls.Add(fn)
}

func (h eventHandlers) addRm(l sync.Locker, typ event.Type, fn interface{}) func() {
	b := h.add(typ, fn)
	return func() {
		l.Lock()
		b.Delete()
		l.Unlock()
	}
}

func invokeSync(r registry.M, sync *api.SyncResponse) {
	for v := range r {
		v.V.(func(*api.SyncResponse))(sync)
	}
}

type eventInvoker struct {
	raw    *event.RawEvent
	parsed event.Event
}

func eventInvoke(rID matrix.RoomID, raw *event.RawEvent) eventInvoker {
	if raw.RoomID == "" && rID != "" {
		raw.RoomID = rID
	}

	return eventInvoker{raw: raw}
}

func (i *eventInvoker) parse() (event.Event, error) {
	if i.parsed != nil {
		return i.parsed, nil
	}

	p, err := i.raw.Parse()
	if err != nil {
		return nil, err
	}

	i.parsed = p
	return p, nil
}

func (i *eventInvoker) invokeList(list registry.M) {
	if list == nil {
		return
	}

	for v := range list {
		i.invoke(v.V)
	}
}

func (i *eventInvoker) invoke(f interface{}) {
	switch fn := f.(type) {
	case func(event.Event):
		v, err := i.parse()
		if err != nil {
			return
		}
		fn(v)
	case func(event.RoomEvent):
		v, err := i.parse()
		if err != nil {
			return
		}
		rv, ok := v.(event.RoomEvent)
		if !ok {
			return
		}
		fn(rv)
	case func(event.StateEvent):
		v, err := i.parse()
		if err != nil {
			return
		}
		sv, ok := v.(event.StateEvent)
		if !ok {
			return
		}
		fn(sv)
	case func(*event.RawEvent):
		fn(i.raw)
	case func(*eventInvoker):
		fn(i)
	case func():
		fn()
	default:
		log.Panicf("BUG: unknown handler type %T", fn)
	}
}
