package handler

import (
	"log"
	"sync"

	"github.com/diamondburned/gotktrix/internal/registry"
	"github.com/diamondburned/gotrix"
	"github.com/diamondburned/gotrix/api"
	"github.com/diamondburned/gotrix/event"
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

type handlerMeta struct {
	once bool
}

type eventHandlers struct {
	regs map[event.Type]registry.Registry
	mut  sync.Locker
}

func newEventHandlers(mut sync.Locker, cap int) eventHandlers {
	return eventHandlers{
		regs: make(map[event.Type]registry.Registry, cap),
		mut:  mut,
	}
}

func (h eventHandlers) invoke(ev event.Event) {
	invokeList(ev, h.regs[ev.Info().Type])
	invokeList(ev, h.regs["*"])
}

func (h eventHandlers) addEvsRm(types []event.Type, fn interface{}, meta handlerMeta) func() {
	if types == nil {
		types = []event.Type{"*"}
	}
	if len(types) == 1 {
		return h.addRm(types[0], fn, meta)
	}

	elems := make([]*registry.Value, len(types))
	for i, typ := range types {
		elems[i] = h.add(typ, fn, meta)
	}

	return func() {
		go func() {
			h.mut.Lock()
			defer h.mut.Unlock()

			for _, elem := range elems {
				elem.Delete()
			}
		}()
	}
}

func (h eventHandlers) add(typ event.Type, fn interface{}, meta handlerMeta) *registry.Value {
	ls, ok := h.regs[typ]
	if !ok {
		ls = registry.New(10)
		h.regs[typ] = ls
	}

	return ls.Add(fn, meta)
}

func (h eventHandlers) addRm(typ event.Type, fn interface{}, meta handlerMeta) func() {
	b := h.add(typ, fn, meta)
	return func() {
		go func() {
			h.mut.Lock()
			b.Delete()
			h.mut.Unlock()
		}()
	}
}

func invokeSync(r registry.Registry, sync *api.SyncResponse) {
	r.Each(func(f, _ interface{}) {
		f.(func(*api.SyncResponse))(sync)
	})
}

func invokeHandlers(ev event.Event, handlers eventHandlers) {
	invokeList(ev, handlers.regs[ev.Info().Type])
	invokeList(ev, handlers.regs["*"])
}

func invokeList(ev event.Event, list registry.Registry) {
	list.Each(func(f, _ interface{}) {
		invoke(ev, f)
	})
}

func invoke(ev event.Event, f interface{}) {
	switch fn := f.(type) {
	case func(event.Event):
		fn(ev)
	case func(event.RoomEvent):
		rv, ok := ev.(event.RoomEvent)
		if !ok {
			return
		}
		fn(rv)
	case func(event.StateEvent):
		sv, ok := ev.(event.StateEvent)
		if !ok {
			return
		}
		fn(sv)
	case func():
		fn()
	default:
		log.Panicf("BUG: unknown handler type %T", fn)
	}
}
