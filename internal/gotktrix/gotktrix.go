package gotktrix

import (
	"context"
	"sync"

	"github.com/chanbakjsd/gotrix"
	"github.com/chanbakjsd/gotrix/api/httputil"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/state"
	"github.com/pkg/errors"
)

type Client struct {
	*gotrix.Client
	*intern
	State *state.State
}

type intern struct {
	waitMu sync.Mutex
	waits  map[event.Type]map[chan event.Event]struct{}
}

// New wraps around gotrix.NewWithClient.
func New(httpClient httputil.Client, serverName string) (*Client, error) {
	c, err := gotrix.NewWithClient(httpClient, serverName)
	if err != nil {
		return nil, err
	}

	return wrapClient(c)
}

// Discover wraps around gotrix.DiscoverWithClient.
func Discover(httpClient httputil.Client, serverName string) (*Client, error) {
	c, err := gotrix.DiscoverWithClient(httpClient, serverName)
	if err != nil {
		return nil, err
	}

	return wrapClient(c)
}

func wrapClient(c *gotrix.Client) (*Client, error) {
	logInit()

	state, err := state.New(config.Path("matrix-state"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to make state db")
	}

	c.State = state
	client := Client{
		Client: c,
		State:  state,
		intern: &intern{
			waits: make(map[event.Type]map[chan event.Event]struct{}),
		},
	}

	client.AddHandler(func(_ *gotrix.Client, raw *event.RawEvent) {
		if !client.waitingForEvent(raw.Type) {
			return
		}

		e, err := raw.Parse()
		if err != nil {
			return
		}

		client.waitMu.Lock()
		defer client.waitMu.Unlock()

		chMap := client.waits[raw.Type]
		for ch := range chMap {
			ch <- e
			delete(chMap, ch)
		}
	})

	return &client, nil
}

// Open opens the client with the last next batch string.
func (c *Client) Open() error {
	next, _ := c.State.NextBatch()
	return c.Client.OpenWithNext(next)
}

func (c *Client) Close() error {
	err1 := c.Client.Close()
	err2 := c.State.Close()

	if err1 != nil {
		return err1
	}
	return err2
}

func (c *Client) WithContext(ctx context.Context) *Client {
	return &Client{
		Client: c.Client.WithContext(ctx),
		State:  c.State,
		intern: c.intern,
	}
}

func (c *Client) waitingForEvent(typ event.Type) bool {
	c.waitMu.Lock()
	defer c.waitMu.Unlock()

	chMap, ok := c.waits[typ]
	return ok && len(chMap) > 0
}

func (c *Client) addEventCh(typ event.Type, ch chan event.Event) {
	c.waitMu.Lock()
	defer c.waitMu.Unlock()

	evMap := c.waits[typ]
	if evMap == nil {
		evMap = make(map[chan event.Event]struct{})
		c.waits[typ] = evMap
	}

	evMap[ch] = struct{}{}
}

func (c *Client) removeEventCh(typ event.Type, ch chan event.Event) {
	c.waitMu.Lock()
	defer c.waitMu.Unlock()

	evMap, ok := c.waits[typ]
	if ok {
		delete(evMap, ch)
	}
}

// WaitForEvent waits for an event of the given type until the context expires.
// If the event exists in the state, then it is returned.
func (c *Client) WaitForEvent(ctx context.Context, typ event.Type) (event.Event, error) {
	if ev, err := c.State.Event(typ); err == nil {
		return ev, nil
	}

	ch := make(chan event.Event, 1)
	c.addEventCh(typ, ch)

	// Double-check after adding the event channel.
	if ev, err := c.State.Event(typ); err == nil {
		c.removeEventCh(typ, ch)
		return ev, nil
	}

	// No events; use a select channel.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case ev := <-ch:
		return ev, nil
	}
}

// ChForEvent waits for an event and feeds the event into the channel. The
// channel's buffer must AT LEAST be 1; a panic will occur otherwise.
func (c *Client) ChForEvent(typ event.Type, ch chan event.Event) {
	if cap(ch) < 1 {
		panic("given channel is not buffered")
	}

	if ev, err := c.State.Event(typ); err == nil {
		ch <- ev
		return
	}

	c.addEventCh(typ, ch)

	// Double-check after adding the event channel.
	if ev, err := c.State.Event(typ); err == nil {
		// Ensure the event channel is removed.
		c.removeEventCh(typ, ch)
		// If the event channel is not filled, then use the event from the
		// state; otherwise, use the handled event.
		if len(ch) == 0 {
			ch <- ev
		}
		return
	}
}
