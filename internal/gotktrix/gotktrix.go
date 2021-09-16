package gotktrix

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/bits"
	"mime"
	"os"
	"sync"

	"github.com/chanbakjsd/gotrix"
	"github.com/chanbakjsd/gotrix/api"
	"github.com/chanbakjsd/gotrix/api/httputil"
	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/config"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
	"github.com/diamondburned/gotktrix/internal/gotktrix/indexer"
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

var deviceName = "gotktrix"

func init() {
	hostname, err := os.Hostname()
	if err == nil {
		deviceName += " (" + hostname + ")"
	}
}

// EventBox provides a concurrently-safe wrapper around a raw event that caches
// event parsing.
type EventBox struct {
	*event.RawEvent
	parsed event.Event
	error  error
	once   sync.Once
}

// WrapEventBox wraps the given raw event.
func WrapEventBox(raw *event.RawEvent) *EventBox {
	return &EventBox{RawEvent: raw}
}

// Parse parses the raw event.
func (b *EventBox) Parse() (event.Event, error) {
	b.once.Do(func() {
		b.parsed, b.error = b.RawEvent.Parse()
	})
	return b.parsed, b.error
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

type ctxKey uint

const (
	clientCtxKey ctxKey = iota
)

// WithClient injects the given client into a new context.
func WithClient(ctx context.Context, c *Client) context.Context {
	return context.WithValue(ctx, clientCtxKey, c)
}

// FromContext returns the client inside the context wrapped with WithClient. If
// the context isn't yet wrapped, then nil is returned.
func FromContext(ctx context.Context) *Client {
	c, _ := ctx.Value(clientCtxKey).(*Client)
	if c != nil {
		return c.WithContext(ctx)
	}
	return nil
}

// ClientAuth holds a partial client.
type ClientAuth struct {
	c *gotrix.Client
}

// Discover wraps around gotrix.DiscoverWithClienT.
func Discover(hcl httputil.Client, serverName string) (*ClientAuth, error) {
	c, err := gotrix.DiscoverWithClient(hcl, serverName)
	if err != nil {
		return nil, err
	}

	return &ClientAuth{c}, nil
}

// WithContext creates a copy of ClientAuth that uses the provided context.
func (a *ClientAuth) WithContext(ctx context.Context) *ClientAuth {
	return &ClientAuth{c: a.c.WithContext(ctx)}
}

// LoginPassword authenticates the client using the provided username and
// password.
func (a *ClientAuth) LoginPassword(username, password string) (*Client, error) {
	err := a.c.Client.Login(api.LoginArg{
		Type: matrix.LoginPassword,
		Identifier: matrix.Identifier{
			Type: matrix.IdentifierUser,
			User: username,
		},
		Password:                 password,
		InitialDeviceDisplayName: deviceName,
	})
	if err != nil {
		return nil, err
	}
	return wrapClient(a.c)
}

// LoginToken authenticates the client using the provided token.
func (a *ClientAuth) LoginToken(token string) (*Client, error) {
	err := a.c.Client.Login(api.LoginArg{
		Type:                     matrix.LoginToken,
		Token:                    deviceName,
		InitialDeviceDisplayName: deviceName,
	})
	if err != nil {
		return nil, err
	}
	return wrapClient(a.c)
}

type Client struct {
	*gotrix.Client
	*handler.Registry
	State *state.State
	Index *indexer.Indexer

	userID matrix.UserID
}

// New wraps around gotrix.NewWithClient.
func New(hcl httputil.Client, serverName string, uID matrix.UserID, token string) (*Client, error) {
	c, err := gotrix.NewWithClient(hcl, serverName)
	if err != nil {
		return nil, err
	}

	c.UserID = uID
	c.AccessToken = token

	return wrapClient(c)
}

func wrapClient(c *gotrix.Client) (*Client, error) {
	logInit()

	u, err := c.Whoami()
	if err != nil {
		return nil, errors.Wrap(err, "invalid user account")
	}

	// URLEncoding is path-safe; StdEncoding is not.
	b64Username := base64.URLEncoding.EncodeToString([]byte(u))

	state, err := state.New(config.Path("matrix-state", b64Username), u)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make state db")
	}

	idx, err := indexer.Open(config.Path("matrix-index", b64Username))
	if err != nil {
		return nil, errors.Wrap(err, "failed to make indexer")
	}

	c.AddHandler(func(c *gotrix.Client, member event.RoomMemberEvent) {
		b := idx.Begin()
		b.IndexRoomMember(member)
		b.Commit()
	})

	registry := handler.New()

	c.State = registry.Wrap(state)
	c.Filter = Filter

	return &Client{
		Client:   c,
		Registry: registry,
		State:    state,
		Index:    idx,
		userID:   u,
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

// Close closes the event loop and the internal database, as well as halting all
// ongoing requests.
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

// Online returns a Client that uses the given context instead of the cancelled
// context. It is an alias to WithContext; the only difference is that the name
// implies the client may be offline prior to this call.
func (c *Client) Online(ctx context.Context) *Client {
	return c.WithContext(ctx)
}

// WithContext replaces the client's internal context with the given one.
func (c *Client) WithContext(ctx context.Context) *Client {
	return &Client{
		Client:   c.Client.WithContext(ctx),
		Registry: c.Registry,
		State:    c.State,
		Index:    c.Index,
		userID:   c.userID,
	}
}

// Whoami is a cached version of the Whoami method.
func (c *Client) Whoami() (matrix.UserID, error) {
	return c.userID, nil
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

// Thumbnail is a helper function around MediaThumbnailURL. It works similarly
// to SquareThumbnail, except the dimensions are unchanged.
func (c *Client) Thumbnail(mURL matrix.URL, w, h int) (string, error) {
	return c.MediaThumbnailURL(mURL, true, w, h, api.MediaThumbnailCrop)
}

// ScaledThumbnail is like Thumbnaill, except the image URL in the image
// respects the original aspect ratio and not the requested one.
func (c *Client) ScaledThumbnail(mURL matrix.URL, w, h int) (string, error) {
	return c.MediaThumbnailURL(mURL, true, w, h, api.MediaThumbnailScale)
}

// ImageThumbnail gets the thumbnail or direct URL of the image from the
// message.
func (c *Client) ImageThumbnail(msg event.RoomMessageEvent, maxW, maxH int) (string, error) {
	i, err := msg.ImageInfo()
	if err == nil {
		maxW, maxH = MaxSize(i.Width, i.Height, maxW, maxH)

		if i.ThumbnailURL != "" {
			return c.ScaledThumbnail(i.ThumbnailURL, maxW, maxH)
		}
	}

	if msg.MsgType != event.RoomMessageImage {
		return "", errors.New("message is not image")
	}

	return c.ScaledThumbnail(msg.URL, maxW, maxH)
}

// MaxSize returns the maximum size that can fit within the given max width and
// height. Aspect ratio is preserved.
func MaxSize(w, h, maxW, maxH int) (int, int) {
	if w == 0 {
		w = maxW
	}
	if h == 0 {
		h = maxH
	}
	if w < maxW && h < maxH {
		return w, h
	}

	if w > h {
		h = h * maxW / w
		w = maxW
	} else {
		w = w * maxH / h
		h = maxH
	}

	return w, h
}

// MessageMediaURL gets the message's media URL, if any.
func (c *Client) MessageMediaURL(msg event.RoomMessageEvent) (string, error) {
	filename := msg.Body

	if filename == "" {
		i, err := msg.FileInfo()
		if err == nil {
			t, err := mime.ExtensionsByType(i.MimeType)
			if err == nil && t != nil {
				filename = "file" + t[0]
			}
		}
	}

	u, err := c.MediaDownloadURL(msg.URL, true, filename)
	if err != nil {
		return "", errors.Wrap(err, "failed to get download URL")
	}

	return u, nil
}

// RoomEvent queries the event with the given type. If the event type implies a
// state event, then the empty key is tried.
func (c *Client) RoomEvent(roomID matrix.RoomID, typ event.Type) (event.Event, error) {
	return c.State.RoomEvent(roomID, typ)
}

// RoomState queries the internal State for the given RoomEvent. If the State
// does not have that event, it queries the homeserver directly.
func (c *Client) RoomState(
	roomID matrix.RoomID, typ event.Type, key string) (event.StateEvent, error) {

	e, err := c.State.RoomState(roomID, typ, key)
	if err == nil {
		return e, nil
	}

	raw, err := c.Client.Client.RoomState(roomID, typ, key)
	if err != nil {
		return nil, err
	}

	parsed, err := raw.Parse()
	if err != nil {
		return nil, err
	}

	stateEvent, ok := parsed.(event.StateEvent)
	if !ok {
		return nil, gotrix.ErrInvalidStateEvent
	}

	// Update the state cache for future calls.
	c.State.AddRoomEvents(roomID, []event.RawEvent{*raw})

	return stateEvent, nil
}

// RoomIsUnread returns true if the room with the given ID has not been read by
// this user. The result of the unread boolean will always be valid, but if ok
// is false, then it might not be accurate.
func (c *Client) RoomIsUnread(roomID matrix.RoomID) (unread, ok bool) {
	t, err := c.RoomTimeline(roomID)
	if err != nil || len(t) == 0 {
		// Nothing in the timeline. Assume the user has already caught up, since
		// the room is empty.
		return false, false
	}

	seen, ok := c.hasSeenEvent(roomID, t[len(t)-1].ID())
	return !seen, ok
}

func (c *Client) hasSeenEvent(roomID matrix.RoomID, eventID matrix.EventID) (seen, ok bool) {
	e, err := c.RoomEvent(roomID, m.FullyReadEventType)
	if err == nil {
		fullyRead := e.(m.FullyReadEvent)
		// Assume that the user has caught up to the room if the latest event's
		// ID matches. Technically, there shouldn't ever be a case where the
		// fully read event would point to an event in the future, so this
		// should work.
		return fullyRead.EventID == eventID, true
	}

	u, err := c.Whoami()
	if err != nil {
		// Can't get the current user, so just assume that the room is unread.
		// This would be a bug, but whatever.
		return false, false
	}

	e, err = c.RoomEvent(roomID, event.TypeReceipt)
	if err == nil {
		// Query to see if the current user has read the latest message.
		e := e.(event.ReceiptEvent)

		rc, ok := e.Events[eventID]
		if !ok {
			// Nobody has read the latest message, including the current user.
			return false, true
		}

		_, read := rc.Read[u]
		return read, true
	}

	return false, false
}

// MarkRoomAsRead sends to the server that the current user has seen up to the
// given event in the given room.
func (c *Client) MarkRoomAsRead(roomID matrix.RoomID, eventID matrix.EventID) error {
	if seen, ok := c.hasSeenEvent(roomID, eventID); ok && seen {
		// Room is already seen; don't waste an API call.
		return nil
	}

	var request struct {
		FullyRead matrix.EventID `json:"m.fully_read"`
		Read      matrix.EventID `json:"m.read,omitempty"`
	}

	request.FullyRead = eventID
	request.Read = eventID

	return c.Request(
		"POST", api.EndpointRoom(roomID)+"/read_markers",
		nil, httputil.WithToken(), httputil.WithJSONBody(request),
	)
}

// RoomEnsureMembers ensures that the given room has all its members fetched.
func (c *Client) RoomEnsureMembers(roomID matrix.RoomID) error {
	const key = "ensure-members"

	if !c.State.SetRoom(roomID, key) {
		return nil
	}

	p, err := c.State.RoomPreviousBatch(roomID)
	if err != nil {
		c.State.ResetRoom(roomID, key)
		return fmt.Errorf("no previous_batch for room %q found", roomID)
	}

	e, err := c.Client.RoomMembers(roomID, api.RoomMemberFilter{
		At:         p,
		Membership: event.MemberJoined,
	})
	if err != nil {
		c.State.ResetRoom(roomID, key)
		return errors.Wrap(err, "failed to fetch members")
	}

	c.State.AddRoomEvents(roomID, e)

	batch := c.Index.Begin()
	defer batch.Commit()

	for _, raw := range e {
		e, err := raw.Parse()
		if err != nil {
			log.Println("error parsing RoomMembers event:", err)
			continue
		}

		me, ok := e.(event.RoomMemberEvent)
		if !ok {
			log.Printf("error: RoomMember event is of unexpected type %T", e)
			continue
		}

		batch.IndexRoomMember(me)
	}

	return nil
}

// RoomTimeline queries the state cache for the timeline of the given room. If
// it's not available, the API will be queried directly. The order of these
// events is guaranteed to be latest last.
func (c *Client) RoomTimeline(roomID matrix.RoomID) ([]event.RoomEvent, error) {
	rawEvents, err := c.RoomTimelineRaw(roomID)
	if err != nil {
		return nil, err
	}

	events := make([]event.RoomEvent, 0, len(rawEvents))

	for i := range rawEvents {
		rawEvents[i].RoomID = roomID

		e, err := rawEvents[i].Parse()
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

// RoomTimelineRaw is RoomTimeline, except events are returned unparsed.
func (c *Client) RoomTimelineRaw(roomID matrix.RoomID) ([]event.RawEvent, error) {
	if events, err := c.State.RoomTimelineRaw(roomID); err == nil {
		return events, nil
	}

	// Obtain the previous batch.
	prev, err := c.State.RoomPreviousBatch(roomID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get room previous batch")
	}

	// Re-check the state for the timeline, because we don't want to miss out
	// any events whil we were fetching the previous_batch string.
	if events, err := c.State.RoomTimelineRaw(roomID); err == nil {
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

	return r.Chunk, nil
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

// SendRoomEvent is a convenient function around RoomEventSend.
func (c *Client) SendRoomEvent(roomID matrix.RoomID, ev event.Event) error {
	_, err := c.Client.RoomEventSend(roomID, ev.Type(), ev)
	return err
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
//
// If check is true, then the MemberName's Ambiguous field will be set to true
// if the display name collides with someone else's. This check is quite
// expensive, so it should only be enabled when needed.
func (c *Client) MemberName(
	roomID matrix.RoomID, userID matrix.UserID, check bool) (MemberName, error) {

	names, err := c.memberNames(roomID, []matrix.UserID{userID}, check)
	if err != nil {
		return MemberName{}, err
	}

	return names[0], nil
}

// memberNames calculates the display name of all the users provided.
func (c *Client) memberNames(
	roomID matrix.RoomID, userIDs []matrix.UserID, check bool) ([]MemberName, error) {

	results := make([]MemberName, len(userIDs))

	for i, userID := range userIDs {
		e, _ := c.RoomState(roomID, event.TypeRoomMember, string(userID))
		if e == nil {
			results[i].Name = string(userID)
			continue
		}

		memberEvent := e.(event.RoomMemberEvent)
		if memberEvent.DisplayName == nil || *memberEvent.DisplayName == "" {
			results[i].Name = string(userID)
			continue
		}

		results[i].Name = *memberEvent.DisplayName
	}

	if !check {
		return results, nil
	}

	for i, result := range results {
		for _, userID := range c.State.RoomMembersFromName(roomID, result.Name) {
			if userID != userIDs[i] {
				results[i].Ambiguous = true
			}
		}
	}

	return results, nil
}

// UpdateRoomTags updates the internal state with the latest room tag
// information.
func (c *Client) UpdateRoomTags(roomID matrix.RoomID) error {
	t, err := c.Client.Tags(roomID)
	if err != nil {
		return err
	}

	b, err := json.Marshal(event.TagEvent{
		RoomID: roomID,
		Tags:   t,
	})
	if err != nil {
		return errors.Wrap(err, "failed to marshal room tags")
	}

	c.State.AddRoomEvents(roomID, []event.RawEvent{{
		Type:    event.TypeTag,
		Content: b,
		RoomID:  roomID,
	}})

	return nil
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
