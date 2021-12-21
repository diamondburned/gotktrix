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

func (h eventHandlers) invoke(ivk *eventInvoker) {
	ivk.invokeList(h.regs[ivk.raw.Type])
	ivk.invokeList(h.regs["*"])
}

func (h eventHandlers) addEvsRm(types []event.Type, fn interface{}, meta handlerMeta) func() {
	if len(types) == 1 {
		return h.addRm(types[0], fn, meta)
	}

	elems := make([]*registry.Value, len(types))
	for i, typ := range types {
		elems[i] = h.add(typ, fn, meta)
	}

	return func() {
		h.mut.Lock()
		defer h.mut.Unlock()

		for _, elem := range elems {
			elem.Delete()
		}
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
		h.mut.Lock()
		b.Delete()
		h.mut.Unlock()
	}
}

func invokeSync(r registry.Registry, sync *api.SyncResponse) {
	r.Each(func(f, _ interface{}) {
		f.(func(*api.SyncResponse))(sync)
	})
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

func (i *eventInvoker) invokeList(list registry.Registry) {
	list.Each(func(f, _ interface{}) {
		i.invoke(f)
	})
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
