package gotktrix

import (
	"context"
	"sync"

	"github.com/chanbakjsd/gotrix"
	"github.com/chanbakjsd/gotrix/api"
	"github.com/chanbakjsd/gotrix/api/httputil"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/state"
	"github.com/pkg/errors"
)

var Filter = event.GlobalFilter{
	Room: event.RoomFilter{
		IncludeLeave: true,
		State: event.StateFilter{
			LazyLoadMembers: true,
		},
		Timeline: event.RoomEventFilter{
			Limit:           state.TimelineKeepLast,
			LazyLoadMembers: true,
		},
	},
}

var (
	cancelled  context.Context
	cancelOnce sync.Once
)

// Cancelled gets a cancelled context.
func Cancelled() context.Context {
	cancelOnce.Do(func() {
		var cancel func()

		cancelled, cancel = context.WithCancel(context.Background())
		cancel()
	})

	return cancelled
}

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

// Discover wraps around gotrix.DiscoverWithClienT.
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
	c.Filter = Filter

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

// Offline returns a Client that does not use the API.
func (c *Client) Offline() *Client {
	return c.WithContext(Cancelled())
}

func (c *Client) WithContext(ctx context.Context) *Client {
	return &Client{
		Client: c.Client.WithContext(ctx),
		State:  c.State,
		intern: c.intern,
	}
}

// Whoami is a cached version of the Whoami method.
func (c *Client) Whoami() (matrix.UserID, error) {
	u, err := c.State.Whoami()
	if err == nil {
		return u, nil
	}

	u, err = c.Client.Whoami()
	if err != nil {
		return "", err
	}

	// TODO: cache stampede problem, yadda yadda. That shouldn't even be a
	// problem most of the time if the method is called before anything else.

	c.State.SetWhoami(u)
	return u, nil
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

// WaitForUserEvent waits for a user event of the given type until the context
// expires. If the event exists in the state, then it is returned.
func (c *Client) WaitForUserEvent(ctx context.Context, typ event.Type) (event.Event, error) {
	if ev, err := c.State.UserEvent(typ); err == nil {
		return ev, nil
	}

	ch := make(chan event.Event, 1)
	c.addEventCh(typ, ch)

	// Double-check after adding the event channel.
	if ev, err := c.State.UserEvent(typ); err == nil {
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

	if ev, err := c.State.UserEvent(typ); err == nil {
		ch <- ev
		return
	}

	c.addEventCh(typ, ch)

	// Double-check after adding the event channel.
	if ev, err := c.State.UserEvent(typ); err == nil {
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

// SquareThumbnail is a helper function around MediaThumbnailURL.
func (c *Client) SquareThumbnail(mURL matrix.URL, size int) (string, error) {
	return c.MediaThumbnailURL(mURL, true, size, size, api.MediaThumbnailCrop)
}

// RoomTimeline queries the state cache for the timeline of the given room. If
// it's not available, the API will be queried directly. The order of these
// events is guaranteed to be latest last.
func (c *Client) RoomTimeline(roomID matrix.RoomID) ([]event.RoomEvent, error) {
	if events, err := c.State.RoomTimeline(roomID); err == nil {
		return events, nil
	}

	// Obtain the previous batch.
	prev, err := c.State.RoomPreviousBatch(roomID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get room previous batch")
	}

	// Re-check the state for the timeline, because we don't want to miss out
	// any events whil we were fetching the previous_batch string.
	if events, err := c.State.RoomTimeline(roomID); err == nil {
		return events, nil
	}

	r, err := c.RoomMessages(roomID, api.RoomMessagesQuery{
		From:      prev,
		Direction: api.RoomMessagesBackward, // latest last
		Limit:     state.TimelineKeepLast,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get messages for room %q", roomID)
	}

	c.State.AddRoomMessages(roomID, &r)

	// Returned events will be sorted latest-first, so we reverse the slice.
	// https://github.com/golang/go/wiki/SliceTricks#reversing
	for i := len(r.Chunk)/2 - 1; i >= 0; i-- {
		opp := len(r.Chunk) - 1 - i
		r.Chunk[i], r.Chunk[opp] = r.Chunk[opp], r.Chunk[i]
	}

	timelineEvs := make([]event.RoomEvent, 0, len(r.Chunk))

	for i := range r.Chunk {
		e, err := r.Chunk[i].Parse()
		if err != nil {
			continue
		}

		state, ok := e.(event.RoomEvent)
		if ok {
			timelineEvs = append(timelineEvs, state)
		}
	}

	return timelineEvs, nil
}

// LatestMessage finds the latest room message event from the given list of
// events. The list is assumed to have the latest events last.
func LatestMessage(events []event.RoomEvent) (event.RoomMessageEvent, bool) {
	for i := len(events) - 1; i >= 0; i-- {
		msg, ok := events[i].(event.RoomMessageEvent)
		if ok {
			return msg, true
		}
	}
	return event.RoomMessageEvent{}, false
}

// UserEvent gets the user event from the state or the API.
func (c *Client) UserEvent(typ event.Type) (event.Event, error) {
	ev, err := c.State.UserEvent(typ)
	if err != nil {
		return nil, err
	}

	uID, err := c.Whoami()
	if err != nil {
		return nil, errors.Wrap(err, "whoami error")
	}

	raw := event.RawEvent{Type: typ}

	if err := c.ClientConfig(uID, string(typ), &raw.Content); err != nil {
		return nil, errors.Wrap(err, "failed to get client config")
	}

	ev, err = raw.Parse()
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse event from API")
	}

	return ev, nil
}

// Rooms returns the list of rooms the user is in.
func (c *Client) Rooms() ([]matrix.RoomID, error) {
	if roomIDs, err := c.State.Rooms(); err == nil {
		return roomIDs, nil
	}

	return c.Client.Rooms()
}

// RoomMembers returns a list of room members.
func (c *Client) RoomMembers(roomID matrix.RoomID) ([]event.RoomMemberEvent, error) {
	var events []event.RoomMemberEvent

	onEach := func(e event.Event, total int) error {
		ev, ok := e.(event.RoomMemberEvent)
		if !ok {
			return nil
		}

		if events == nil {
			events = make([]event.RoomMemberEvent, 0, total)
		}

		events = append(events, ev)
		return nil
	}

	if err := c.State.EachRoomState(roomID, event.TypeRoomMember, onEach); err == nil {
		if events != nil {
			return events, nil
		}
	}

	// prev is optional.
	prev, _ := c.State.RoomPreviousBatch(roomID)

	return c.Client.RoomMembers(roomID, api.RoomMemberFilter{At: prev})
}
