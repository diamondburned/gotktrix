package compose

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
	"github.com/diamondburned/gotktrix/internal/app/messageview/compose/autocomplete"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/md"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
	"github.com/pkg/errors"
)

// InputController extends Controller to provide additional state getters.
type InputController interface {
	Controller
	// IsEditing returns true if we're currently editing a message.
	IsEditing() bool
}

// Input is the input component of the message composer.
type Input struct {
	*gtk.TextView
	buffer  *gtk.TextBuffer
	acomp   *autocomplete.Autocompleter
	anchors list.List // T = anchorPiece

	ctx    context.Context
	ctrl   InputController
	roomID matrix.RoomID

	inputState
}

type inputState struct {
	editing    matrix.EventID
	replyingTo matrix.EventID
}

type anchorPiece struct {
	anchor *gtk.TextChildAnchor
	html   string
	text   string
}

var inputCSS = cssutil.Applier("composer-input", `
	.composer-input,
	.composer-input text {
		background-color: inherit;
	}
	.composer-input {
		padding: 0px 2px;
		padding-top: 12px;
		margin-top:  0px;
	}
`)

// NewInput creates a new Input instance.
func NewInput(ctx context.Context, ctrl InputController, roomID matrix.RoomID) *Input {
	go requestAllMembers(ctx, roomID)

	i := Input{
		ctx:    ctx,
		ctrl:   ctrl,
		roomID: roomID,
	}

	i.TextView = gtk.NewTextView()
	i.TextView.SetWrapMode(gtk.WrapWordChar)
	i.TextView.SetAcceptsTab(true)
	i.TextView.SetHExpand(true)
	i.TextView.SetInputHints(0 |
		gtk.InputHintEmoji |
		gtk.InputHintSpellcheck |
		gtk.InputHintWordCompletion |
		gtk.InputHintUppercaseSentences,
	)
	textutil.SetTabSize(i.TextView)
	inputCSS(i)

	i.acomp = autocomplete.New(ctx, i.TextView, i.onAutocompleted)
	i.acomp.SetTimeout(time.Second)
	i.acomp.Use(
		autocomplete.NewRoomMemberSearcher(ctx, roomID), // @
		autocomplete.NewEmojiSearcher(ctx, roomID),      // :
	)

	i.buffer = i.TextView.Buffer()

	i.buffer.ConnectChanged(func() {
		md.WYSIWYG(ctx, i.buffer)
		i.acomp.Autocomplete()
	})

	i.buffer.ConnectDeleteRange(func(start, end *gtk.TextIter) {
		startOffset := start.Offset()
		endOffset := end.Offset()

		for elem := i.anchors.Front(); elem != nil; elem = elem.Next() {
			anchor := elem.Value.(anchorPiece)
			if anchor.anchor.Deleted() {
				continue
			}

			anIter := i.buffer.IterAtChildAnchor(anchor.anchor)
			offset := anIter.Offset()

			if startOffset <= offset && offset <= endOffset {
				// Deleting the anchor, so remove it off.
				i.anchors.Remove(elem)
			}
		}
	})

	enterKeyer := gtk.NewEventControllerKey()
	enterKeyer.ConnectKeyPressed(i.onKey)
	i.AddController(enterKeyer)

	uploader := uploader{ctx, ctrl, roomID}
	i.ConnectPasteClipboard(uploader.paste)

	return &i
}

func (i *Input) onAutocompleted(row autocomplete.SelectedData) bool {
	i.buffer.BeginUserAction()
	defer i.buffer.EndUserAction()

	// Delete the inserted text, which will equalize the two boundi. The
	// caller will use bounds[1], so we use that to revalidate it.
	i.buffer.Delete(row.Bounds[0], row.Bounds[1])

	switch data := row.Data.(type) {
	case autocomplete.RoomMemberData:
		chip := mauthor.NewChip(i.ctx, data.Room, data.ID)
		anchor := chip.InsertText(i.TextView, row.Bounds[1])

		// Register the anchor.
		i.anchors.PushBack(anchorPiece{
			anchor: anchor,
			html: fmt.Sprintf(
				`<a href="https://matrix.to/#/%s">%s</a>`,
				html.EscapeString(string(data.ID)), html.EscapeString(chip.Name()),
			),
			text: string(data.ID),
		})

	case autocomplete.EmojiData:
		if data.Unicode != "" {
			// Unicode emoji means we can just insert it in plain text.
			i.buffer.Insert(row.Bounds[1], data.Unicode)
		} else {
			anchor := i.buffer.CreateChildAnchor(row.Bounds[1])

			image := md.InsertImageWidget(i.TextView, anchor)
			image.AddCSSClass("compose-inline-emoji")
			image.SetSizeRequest(inlineEmojiSize, inlineEmojiSize)
			image.SetName(data.Name)

			client := gotktrix.FromContext(i.ctx).Offline()
			url, _ := client.SquareThumbnail(data.Custom.URL, inlineEmojiSize, gtkutil.ScaleFactor())
			imgutil.AsyncGET(i.ctx, url, imgutil.ImageSetter{
				SetFromPaintable: image.SetFromPaintable,
				SetFromPixbuf:    image.SetFromPixbuf,
			})

			// Register the anchor.
			i.anchors.PushBack(anchorPiece{
				anchor: anchor,
				html:   customEmojiHTML(data),
				text:   data.Name,
			})
		}
	default:
		log.Printf("unknown data type %T", data)
		return false
	}

	return true
}

func (i *Input) onKey(val, _ uint, state gdk.ModifierType) bool {
	switch val {
	case gdk.KEY_Return:
		if i.acomp.Select() {
			return true
		}

		// TODO: find a better way to do this. goldmark won't try to
		// parse an incomplete codeblock (I think), but the changed
		// signal will be fired after this signal.
		//
		// Perhaps we could use the FindChar method to avoid allocating
		// a new string (twice) on each keypress.
		head := i.buffer.StartIter()
		tail := i.buffer.IterAtOffset(i.buffer.ObjectProperty("cursor-position").(int))
		uinput := i.Text(head, tail)

		// Check if the number of triple backticks is odd. If it is, then we're
		// in one.
		withinCodeblock := strings.Count(uinput, "```")%2 != 0

		// Enter (without holding Shift) sends the message.
		if !state.Has(gdk.ShiftMask) && !withinCodeblock {
			return i.Send()
		}
	case gdk.KEY_Tab:
		return i.acomp.Select()
	case gdk.KEY_Escape:
		if i.ctrl.IsEditing() {
			i.ctrl.Edit("")
			return true
		}
		return i.acomp.Clear()
	case gdk.KEY_Up:
		if i.acomp.MoveUp() {
			return true
		}
		if i.buffer.CharCount() == 0 {
			// Scan for the user's latest message and edit that, if there's any.
			if eventID := i.ctrl.FocusLatestUserEventID(); eventID != "" {
				i.TextView.GrabFocus()
				i.ctrl.Edit(eventID)
				return true
			}
		}
	case gdk.KEY_Down:
		return i.acomp.MoveDown()
	}

	return false
}

// SetText sets the given text (in raw Markdown format, preferably) into the
// input buffer and emits the right signals to render it.
func (i *Input) SetText(text string) {
	start, end := i.buffer.Bounds()

	i.buffer.Delete(start, end)
	i.buffer.Insert(start, text)
}

// HTML returns the Input's content as HTML.
func (i *Input) HTML(start, end *gtk.TextIter) string {
	return i.renderAnchors(start, end, func(anchor anchorPiece) string { return anchor.html })
}

// Text returns the Input's content as plain text.
func (i *Input) Text(start, end *gtk.TextIter) string {
	return i.renderAnchors(start, end, func(anchor anchorPiece) string { return anchor.text })
}

func (i *Input) renderAnchors(start, end *gtk.TextIter, f func(anchorPiece) string) string {
	if i.anchors.Len() == 0 {
		return i.buffer.Text(start, end, true)
	}

	buf := strings.Builder{}
	buf.Grow(end.Offset())

	// Construct a fast lookup map from offsets to strings.
	anchors := make(map[int]string, i.anchors.Len())

	for elem := i.anchors.Front(); elem != nil; elem = elem.Next() {
		anchor := elem.Value.(anchorPiece)
		if !anchor.anchor.Deleted() {
			anIter := i.buffer.IterAtChildAnchor(anchor.anchor)
			anchors[anIter.Offset()] = f(anchor)
		}
	}

	// Use a new iterator to iterate over the whole text buffer.
	iter := start.Copy()
	// Borrow the start iterator to iterate over the whole text buffer. We're
	// skipping invisible positions, because those are for plain text.
	for ok := true; ok; ok = iter.ForwardChar() {
		r := rune(iter.Char())
		if r != '\uFFFC' {
			// Rune is not a Unicode unknown character, so skip the anchor
			// check.
			buf.WriteRune(r)
			continue
		}

		// Check the anchor on this position.
		s, ok := anchors[iter.Offset()]
		if ok {
			buf.WriteString(s)
		} else {
			// Preserve the rune if this isn't our anchor.
			buf.WriteRune(r)
		}
	}

	return buf.String()
}

// Send sends the message inside the input off.
func (i *Input) Send() bool {
	dt, ok := i.put()
	if !ok {
		return false
	}

	ctx := i.ctx
	go func() {
		client := gotktrix.FromContext(ctx)
		roomEv := dt.put(client)

		var eventID matrix.EventID
		var err error

		// Only push a new message if we're not editing.
		if dt.editing == "" {
			rowCh := make(chan interface{}, 1)
			glib.IdleAdd(func() {
				// Give the controller the RoomMessageEvent instead of our
				// private type.
				rowCh <- i.ctrl.AddSendingMessage(&roomEv.RoomMessageEvent)
			})

			defer func() {
				row := <-rowCh
				glib.IdleAdd(func() {
					i.ctrl.BindSendingMessage(row, eventID)
				})
			}()
		}

		eventID, err = client.RoomEventSend(roomEv.RoomID, roomEv.Type, roomEv)
		if err != nil {
			app.Error(i.ctx, errors.Wrap(err, "failed to send message"))
		}
	}()

	i.buffer.Delete(i.buffer.Bounds())

	// Ask the parent to reset the state.
	i.ctrl.ReplyTo("")
	i.ctrl.Edit("")
	return true
}

// put steals the buffer and puts it into a message event. If the buffer is
// empty, then an empty data and false are returned.
func (i *Input) put() (inputData, bool) {
	head, tail := i.buffer.Bounds()

	// TODO: ideally, if we want to get the previous input, we'd want a way to
	// either re-render the HTML as Markdown and somehow preserve that
	// information in the plain body, or we need a way to preserve that
	// information in the text buffer without the user seeing.
	//
	// Re-rendering the HTML is probably the most backwards-compatible way, but
	// it also involves a LOT of effort, and it may not preserve whitespace at
	// all.

	// Get the buffer WITH the invisible HTML segments.
	inputHTML := i.HTML(head, tail)
	// Clean off trailing spaces.
	inputHTML = strings.TrimSpace(inputHTML)

	if inputHTML == "" {
		return inputData{}, false
	}

	// Get the buffer without any invisible segments, since those segments
	// contain HTML.
	plain := i.Text(head, tail)
	// Clean off trailing spaces.
	plain = strings.TrimSpace(plain)

	return inputData{
		roomID:     i.roomID,
		plain:      plain,
		html:       inputHTML,
		inputState: i.inputState,
	}, true
}

type inputData struct {
	roomID matrix.RoomID
	plain  string
	html   string
	inputState
}

type messageEvent struct {
	event.RoomMessageEvent
	NewContent *event.RoomMessageEvent `json:"m.new_content,omitempty"`
}

func newRoomMessageEvent(client *gotktrix.Client, roomID matrix.RoomID) event.RoomMessageEvent {
	return event.RoomMessageEvent{
		RoomEventInfo: event.RoomEventInfo{
			EventInfo: event.EventInfo{
				Type: event.TypeRoomMessage,
			},
			RoomID:           roomID,
			Sender:           client.UserID,
			OriginServerTime: matrix.Timestamp(time.Now().UnixMilli()),
		},
	}
}

// put creates a message event from the input data. It might query the API for
// the information that it needs.
func (data inputData) put(client *gotktrix.Client) *messageEvent {
	ev := messageEvent{RoomMessageEvent: newRoomMessageEvent(client, data.roomID)}
	ev.MessageType = event.RoomMessageText
	ev.RelatesTo = data.relatesTo()

	var html strings.Builder
	var plain strings.Builder

	if data.replyingTo != "" {
		replEv := roomTimelineEvent(client, data.roomID, data.replyingTo)

		if msg, ok := replEv.(*event.RoomMessageEvent); ok {
			renderReply(&html, &plain, client, msg)
		}
	}

	plain.WriteString(data.plain)
	ev.Body = plain.String()

	if err := md.Converter.Convert([]byte(data.html), &html); err == nil {
		var out string
		out = html.String()
		out = strings.TrimSpace(out)

		// Trim off the paragraph tags if we only have 1 pair of it wrapped
		// around the block.
		singleParagraph := true &&
			// check if surrounded by p tags
			strings.HasPrefix(out, "<p>") && strings.HasSuffix(out, "</p>") &&
			// check that there's only 1 pair of p tags
			strings.Count(out, "<p>") == 1 && strings.Count(out, "</p>") == 1

		if singleParagraph {
			out = strings.TrimPrefix(out, "<p>")
			out = strings.TrimSuffix(out, "</p>")
		}

		ev.Format = event.FormatHTML
		ev.FormattedBody = out
	}

	// If we're editing an existing message, then insert a new_content object.
	if data.editing != "" {
		ev.NewContent = &event.RoomMessageEvent{
			Body:          ev.Body,
			MessageType:   ev.MessageType,
			Format:        ev.Format,
			FormattedBody: ev.FormattedBody,
		}
		// We should also append a "*" into the outside body to indicate by
		// conventional means that the message is an edit.
		if ev.Body != "" {
			ev.Body += "*"
		}
		if ev.FormattedBody != "" {
			ev.FormattedBody += "*"
		}
	}

	return &ev
}

func (data inputData) relatesTo() json.RawMessage {
	if data.inputState == (inputState{}) {
		return nil
	}

	type inReplyTo struct {
		EventID matrix.EventID `json:"event_id"`
	}

	var relatesTo struct {
		EventID   matrix.EventID `json:"event_id,omitempty"`
		RelType   string         `json:"rel_type,omitempty"`
		InReplyTo *inReplyTo     `json:"m.in_reply_to,omitempty"`
	}

	if data.editing != "" {
		relatesTo.EventID = data.editing
		relatesTo.RelType = "m.replace"
	}

	if data.replyingTo != "" {
		relatesTo.InReplyTo = &inReplyTo{
			EventID: data.replyingTo,
		}
	}

	b, err := json.Marshal(relatesTo)
	if err != nil {
		log.Panicf("error marshaling relatesTo: %v", err) // bug
	}

	return b
}

const inlineEmojiSize = 18

func customEmojiHTML(emoji autocomplete.EmojiData) string {
	if emoji.Unicode != "" {
		return emoji.Unicode
	}

	return fmt.Sprintf(
		`<img alt="%s" title="%[1]s" width="32" height="32" src="%s" data-mx-emoticon />`,
		html.EscapeString(string(emoji.Name)),
		html.EscapeString(string(emoji.Custom.URL)),
	)
}

// requestAllMembers asynchronously fills up the local state with the given
// room's members.
func requestAllMembers(ctx context.Context, roomID matrix.RoomID) {
	client := gotktrix.FromContext(ctx)

	if err := client.RoomEnsureMembers(roomID); err != nil {
		app.Error(ctx, errors.Wrap(err, "failed to prefetch members"))
	}
}
