package gotktrix

import (
	"context"
	"log"
	"math/bits"
	"sync"

	"github.com/chanbakjsd/gotrix"
	"github.com/chanbakjsd/gotrix/api"
	"github.com/chanbakjsd/gotrix/api/httputil"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/handler"
	"github.com/diamondburned/gotktrix/internal/gotktrix/internal/state"
	"github.com/pkg/errors"
)

// TimelimeLimit is the number of timeline events that the database keeps.
const TimelimeLimit = state.TimelineKeepLast

var Filter = event.GlobalFilter{
	Room: event.RoomFilter{
		IncludeLeave: true,
		State: event.StateFilter{
			LazyLoadMembers: true,
		},
		Timeline: event.RoomEventFilter{
			Limit:           TimelimeLimit,
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
	*handler.Registry
	State *state.State
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

	registry := handler.New()

	c.State = registry.Wrap(state)
	c.Filter = Filter

	return &Client{
		Client:   c,
		Registry: registry,
		State:    state,
	}, nil
}

// AddHandler will panic.
//
// Deprecated: Use c.On() instead.
func (c *Client) AddHandler(function interface{}) error {
	panic("don't use AddHandler(); use On().")
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

// Online returns a Client that can use the API. It is meant to be used to
// guarantee that a synchronous fetching routine is meaningful by using the API.
func (c *Client) Online() *Client {
	return c.WithContext(context.Background())
}

func (c *Client) WithContext(ctx context.Context) *Client {
	return &Client{
		Client:   c.Client.WithContext(ctx),
		Registry: c.Registry,
		State:    c.State,
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

// thumbnailScale determines the sacle at which square/round thumbnails will be
// fetched. It's mostly important for HiDPI displays. Note that the dimensions
// are scaled up to the next power of 2 as well, so for example, 38px will end
// up being 128px.
const thumbnailScale = 2

// roundPow2 rounds x up to the nearest power of 2. For example, if 36 is given,
// then the returned number is 64.
func roundPow2(x uint) uint {
	return 1 << bits.Len(x-1)
}

// SquareThumbnail is a helper function around MediaThumbnailURL. The given size
// is assumed to be a square, and the size will be scaled up to the next power
// of 2 and multiplied up for ensured HiDPI support of up to 2x.
func (c *Client) SquareThumbnail(mURL matrix.URL, size int) (string, error) {
	size = int(roundPow2(uint(size)))
	size = size * thumbnailScale

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
		Direction: api.RoomMessagesForward, // latest last
		Limit:     state.TimelineKeepLast,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get messages for room %q", roomID)
	}

	// c.State.AddRoomMessages(roomID, &r)

	events := make([]event.RoomEvent, 0, len(r.Chunk))

	for i := range r.Chunk {
		r.Chunk[i].RoomID = roomID

		e, err := r.Chunk[i].Parse()
		if err != nil {
			continue
		}

		state, ok := e.(event.RoomEvent)
		if ok {
			events = append(events, state)
		}
	}

	return events, nil
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

// AsyncSetConfig updates the state cache first, and then updates the API in the
// background.
//
// If done is given, then it's called once the API is updated. Most of the time,
// done should only be used to display errors; to know when things are updated,
// use a handler. Because of that, done may be invoked before AsyncConfigSet has
// been returned when there's an error. Done might also be called in a different
// goroutine.
func (c *Client) AsyncSetConfig(ev event.Event, done func(error)) {
	if err := c.State.SetUserEvent(ev); err != nil {
		done(err)
		return
	}

	go func() {
		err := c.ClientConfigSet(string(ev.Type()), ev)

		if done != nil {
			done(err)
		}
	}()
}

// UserEvent gets the user event from the state or the API.
func (c *Client) UserEvent(typ event.Type) (event.Event, error) {
	e, _ := c.State.UserEvent(typ)
	if e != nil {
		return e, nil
	}

	raw := event.RawEvent{Type: typ}

	if err := c.ClientConfig(string(typ), &raw.Content); err != nil {
		return nil, errors.Wrap(err, "failed to get client config")
	}

	e, err := raw.Parse()
	if err != nil {
		log.Printf("failed to parse UserEvent %s: %v", typ, err)
		return nil, errors.Wrap(err, "failed to parse event from API")
	}

	return e, nil
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

	if err := c.State.EachRoomStateLen(roomID, event.TypeRoomMember, onEach); err == nil {
		if events != nil {
			return events, nil
		}
	}

	// prev is optional.
	prev, _ := c.State.RoomPreviousBatch(roomID)

	rawEvs, err := c.Client.RoomMembers(roomID, api.RoomMemberFilter{At: prev})
	if err != nil {
		return nil, errors.Wrap(err, "failed to query RoomMembers from API")
	}

	// Save the obtained events into the state cache.
	c.State.AddRoomEvents(roomID, rawEvs)

	events = make([]event.RoomMemberEvent, 0, len(rawEvs))

	for i := range rawEvs {
		rawEvs[i].RoomID = roomID

		e, err := rawEvs[i].Parse()
		if err != nil {
			continue
		}

		ev, ok := e.(event.RoomMemberEvent)
		if !ok {
			continue
		}

		events = append(events, ev)
	}

	return events, nil
}

// MemberName describes a member name.
type MemberName struct {
	Name      string
	Ambiguous bool
}

// MemberName calculates the display name of a member. Note that a user joining
// might invalidate some names if they share the same display name as
// disambiguation will become necessary.
//
// Use the Client.MemberNames variant when generating member name for multiple
// users to reduce duplicate work.
func (c *Client) MemberName(roomID matrix.RoomID, userID matrix.UserID) (MemberName, error) {
	names, err := c.MemberNames(roomID, []matrix.UserID{userID})
	if err != nil {
		return MemberName{}, err
	}
	return names[0], nil
}

// MemberNames calculates the display name of all the users provided.
func (c *Client) MemberNames(roomID matrix.RoomID, userIDs []matrix.UserID) ([]MemberName, error) {
	result := make([]MemberName, len(userIDs))

	for i, userID := range userIDs {
		e, _ := c.RoomState(roomID, event.TypeRoomMember, string(userID))
		if e == nil {
			result[i].Name = string(userID)
			continue
		}

		memberEvent := e.(event.RoomMemberEvent)
		if memberEvent.DisplayName == nil || *memberEvent.DisplayName == "" {
			result[i].Name = string(userID)
			continue
		}

		result[i].Name = *memberEvent.DisplayName
	}

	// Hash all names to check for duplicates.
	names := make(map[string]int, len(userIDs))

	for i, name := range result {
		// Mark any collisions within the given user list.
		if j, ok := names[name.Name]; ok {
			result[j].Ambiguous = true
		}

		// This will override the collided user, if any, but since we've already
		// marked it, we should be fine.
		names[name.Name] = i
	}

	onMember := func(v event.Event, _ int) error {
		ev, ok := v.(event.RoomMemberEvent)
		if !ok || ev.DisplayName == nil {
			return nil
		}

		if i, ok := names[*ev.DisplayName]; ok {
			name := &result[i]

			if !name.Ambiguous && userIDs[i] != ev.UserID {
				// Collide. Mark as ambiguous.
				name.Ambiguous = true
			}
		}

		return nil
	}

	// Reiterate and check for ambiguous names. Ambiguous checking isn't super
	// important, so we can skip it.
	c.State.EachRoomStateLen(roomID, event.TypeRoomMember, onMember)

	return result, nil
}

// IsDirect returns true if the given room is a direct messaging room.
func (c *Client) IsDirect(roomID matrix.RoomID) bool {
	if is, ok := c.State.IsDirect(roomID); ok {
		return is
	}

	if e, err := c.Client.DMRooms(); err == nil {
		c.State.UseDirectEvent(e)
		return roomIsDM(e, roomID)
	}

	u, err := c.Whoami()
	if err != nil {
		return false
	}

	// Resort to querying the room state directly from the API. State.IsDirect
	// already queries RoomState on itself, so we don't need to do that.
	r, err := c.Client.Client.RoomState(roomID, event.TypeRoomMember, string(u))
	if err != nil {
		return false
	}

	r.RoomID = roomID

	// Save the event we've fetched into the state.
	c.State.AddRoomEvents(roomID, []event.RawEvent{*r})

	e, err := r.Parse()
	if err != nil {
		return false
	}

	ev, _ := e.(event.RoomMemberEvent)
	return ev.IsDirect
}

func roomIsDM(dir event.DirectEvent, roomID matrix.RoomID) bool {
	for _, ids := range dir {
		for _, id := range ids {
			if id == roomID {
				return true
			}
		}
	}
	return false
}
