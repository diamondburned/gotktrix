package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"mime"
	"strings"
	"time"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/compose/autocomplete"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
	"github.com/diamondburned/gotktrix/internal/md"
	"github.com/pkg/errors"
)

// Input is the input component of the message composer.
type Input struct {
	*gtk.TextView
	buffer *gtk.TextBuffer
	ac     *autocomplete.Autocompleter

	ctx    context.Context
	ctrl   Controller
	roomID matrix.RoomID

	inputState
}

type inputState struct {
	editing    matrix.EventID
	replyingTo matrix.EventID
}

var inputCSS = cssutil.Applier("composer-input", `
	.composer-input,
	.composer-input text {
		background-color: inherit;
	}
	.composer-input {
		padding: 0px 2px;
		margin-top: 12px;
		margin-top: 6px;
	}
`)

func customEmojiHTML(emoji autocomplete.EmojiData) string {
	if emoji.Unicode != "" {
		return emoji.Unicode
	}

	return fmt.Sprintf(
		`<img alt="%s" title="%[1]s" width="32" height="32" src="%s" data-mxc-emoticon/>`,
		html.EscapeString(string(emoji.Name)),
		html.EscapeString(string(emoji.Custom.URL)),
	)
}

const inlineEmojiSize = 18

// NewInput creates a new Input instance.
func NewInput(ctx context.Context, ctrl Controller, roomID matrix.RoomID) *Input {
	go requestAllMembers(ctx, roomID)

	tview := gtk.NewTextView()
	tview.SetWrapMode(gtk.WrapWordChar)
	tview.SetAcceptsTab(true)
	tview.SetHExpand(true)
	tview.SetInputHints(0 |
		gtk.InputHintEmoji |
		gtk.InputHintSpellcheck |
		gtk.InputHintWordCompletion |
		gtk.InputHintUppercaseSentences,
	)

	md.SetTabSize(tview)
	inputCSS(tview)

	buffer := tview.Buffer()

	ac := autocomplete.New(tview, func(row autocomplete.SelectedData) bool {
		return finishAutocomplete(ctx, tview, buffer, row)
	})
	ac.SetTimeout(time.Second)
	ac.Use(
		autocomplete.NewRoomMemberSearcher(ctx, roomID), // @
		autocomplete.NewEmojiSearcher(ctx, roomID),      // :
	)

	// Ugh. We have to be EXTREMELY careful with this context, because if it's
	// misused, it will put the input buffer into a very inconsistent state.
	// It must be invalidated every time to buffer changes, because we don't
	// want to risk

	buffer.Connect("changed", func(buffer *gtk.TextBuffer) {
		md.WYSIWYG(ctx, buffer)
		ac.Autocomplete(ctx)
	})

	enterKeyer := gtk.NewEventControllerKey()
	tview.AddController(enterKeyer)

	tview.Connect("paste-clipboard", func() {
		display := gdk.DisplayGetDefault()

		clipboard := display.Clipboard()
		clipboard.ReadAsync(ctx, clipboard.Formats().MIMETypes(), 0, func(res gio.AsyncResulter) {
			typ, stream, err := clipboard.ReadFinish(res)
			if err != nil {
				app.Error(ctx, errors.Wrap(err, "failed to read clipboard"))
				return
			}

			mime, _, err := mime.ParseMediaType(typ)
			if err != nil {
				app.Error(ctx, errors.Wrapf(err, "clipboard contains invalid MIME %q", typ))
				return
			}

			// How is utf8_string a valid MIME type? GTK, what the fuck?
			if strings.HasPrefix(mime, "text") || mime == "utf8_string" {
				// Ignore texts.
				stream.Close(ctx)
				return
			}

			log.Println("got mime type", mime)

			promptUpload(ctx, roomID, uploadingFile{
				input:  stream,
				reader: gioutil.Reader(ctx, stream),
				mime:   mime,
				name:   "clipboard",
			})
		})
	})

	input := Input{
		TextView: tview,
		buffer:   buffer,
		ac:       ac,
		ctx:      ctx,
		ctrl:     ctrl,
		roomID:   roomID,
	}

	enterKeyer.Connect("key-pressed", input.onKey)

	return &input
}

func (i *Input) onKey(_ *gtk.EventControllerKey, val, code uint, state gdk.ModifierType) bool {
	switch val {
	case gdk.KEY_Return:
		if i.ac.Select() {
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
		uinput := i.buffer.Text(&head, &tail, false)

		withinCodeblock := strings.Count(uinput, "```")%2 != 0

		// Enter (without holding Shift) sends the message.
		if !state.Has(gdk.ShiftMask) && !withinCodeblock {
			return i.Send()
		}
	case gdk.KEY_Tab:
		return i.ac.Select()
	case gdk.KEY_Escape:
		return i.ac.Clear()
	case gdk.KEY_Up:
		if i.ac.MoveUp() {
			return true
		}
		if i.buffer.CharCount() == 0 {
			// Scan for the user's latest message and edit that, if there's any.
			if eventID := i.ourLatestMessageID(); eventID != "" {
				i.ctrl.Edit(eventID)
				return true
			}
		}
	case gdk.KEY_Down:
		return i.ac.MoveDown()
	}

	return false
}

func (i *Input) ourLatestMessageID() matrix.EventID {
	client := gotktrix.FromContext(i.ctx)
	uID, _ := client.Whoami()
	events, _ := client.RoomTimeline(i.roomID)

	for i := len(events) - 1; i >= 0; i-- {
		ev, ok := events[i].(event.RoomMessageEvent)
		if !ok || ev.SenderID != uID {
			continue
		}
		return ev.EventID
	}

	return ""
}

// SetText sets the given text (in raw Markdown format, preferably) into the
// input buffer and emits the right signals to render it.
func (i *Input) SetText(text string) {
	start, end := i.buffer.Bounds()

	i.buffer.Delete(&start, &end)
	i.buffer.Insert(&start, text, len(text))
}

type messageEvent struct {
	event.RoomMessageEvent
	NewContent *event.RoomMessageEvent `json:"m.new_content,omitempty"`
}

// Send sends the message inside the input off.
func (i *Input) Send() bool {
	dt, ok := i.put()
	if !ok {
		return false
	}

	go func() {
		client := gotktrix.FromContext(i.ctx)
		roomEv := dt.put(client)

		_, err := client.RoomEventSend(roomEv.RoomID, roomEv.Type(), roomEv)
		if err != nil {
			app.Error(i.ctx, errors.Wrap(err, "failed to send message"))
		}
	}()

	head := i.buffer.StartIter()
	tail := i.buffer.EndIter()
	i.buffer.Delete(&head, &tail)

	// Ask the parent to reset the state.
	i.ctrl.ReplyTo("")
	i.ctrl.Edit("")
	return true
}

// put steals the buffer and puts it into a message event. If the buffer is
// empty, then an empty data and false are returned.
func (i *Input) put() (inputData, bool) {
	head := i.buffer.StartIter()
	tail := i.buffer.EndIter()

	// TODO: ideally, if we want to get the previous input, we'd want a way to
	// either re-render the HTML as Markdown and somehow preserve that
	// information in the plain body, or we need a way to preserve that
	// information in the text buffer without the user seeing.
	//
	// Re-rendering the HTML is probably the most backwards-compatible way, but
	// it also involves a LOT of effort, and it may not preserve whitespace at
	// all.

	// Get the buffer WITH the invisible HTML segments.
	inputHTML := i.buffer.Text(&head, &tail, true)
	// Clean off trailing spaces.
	inputHTML = strings.TrimSpace(inputHTML)

	if inputHTML == "" {
		return inputData{}, false
	}

	// Get the buffer without any invisible segments, since those segments
	// contain HTML.
	plain := i.buffer.Text(&head, &tail, false)
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

// put creates a message event from the input data. It might query the API for
// the information that it needs.
func (data inputData) put(client *gotktrix.Client) messageEvent {
	ev := messageEvent{
		RoomMessageEvent: event.RoomMessageEvent{
			RoomEventInfo: event.RoomEventInfo{RoomID: data.roomID},
			Body:          data.plain,
			MsgType:       event.RoomMessageText,
			RelatesTo:     data.relatesTo(),
		},
	}

	var html strings.Builder

	if data.replyingTo != "" {
		replEv := roomTimelineEvent(client, data.roomID, data.replyingTo)

		if msg, ok := replEv.(event.RoomMessageEvent); ok {
			renderReply(&html, client, &msg)
		}
	}

	if err := md.Converter.Convert([]byte(data.html), &html); err == nil {
		var out string
		out = html.String()
		out = strings.TrimSpace(out)

		ev.Format = event.FormatHTML
		ev.FormattedBody = out
	}

	// If we're editing an existing message, then insert a new_content object.
	if data.editing != "" {
		ev.NewContent = &event.RoomMessageEvent{
			Body:          ev.Body,
			MsgType:       ev.MsgType,
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

	return ev
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

func finishAutocomplete(
	ctx context.Context,
	text *gtk.TextView,
	buffer *gtk.TextBuffer,
	row autocomplete.SelectedData) bool {

	buffer.BeginUserAction()
	defer buffer.EndUserAction()

	// Delete the inserted text, which will equalize the two bounds. The
	// caller will use bounds[1], so we use that to revalidate it.
	buffer.Delete(row.Bounds[0], row.Bounds[1])

	// TODO: use TextMarks instead, maybe?

	switch data := row.Data.(type) {
	case autocomplete.RoomMemberData:
		client := gotktrix.FromContext(ctx).Offline()

		md.InsertInvisible(row.Bounds[1], fmt.Sprintf(
			`<a href="https://matrix.to/#/%s">`,
			html.EscapeString(string(data.ID)),
		))
		mauthor.Text(
			client, row.Bounds[1], data.Room, data.ID,
			mauthor.WithWidgetColor(text), mauthor.WithMention(),
		)
		md.InsertInvisible(row.Bounds[1], "</a>")

	case autocomplete.EmojiData:
		if data.Unicode != "" {
			// Unicode emoji means we can just insert it in plain text.
			buffer.Insert(row.Bounds[1], data.Unicode, len(data.Unicode))
		} else {
			// Queue inserting the pixbuf.
			client := gotktrix.FromContext(ctx).Offline()
			url, _ := client.SquareThumbnail(data.Custom.URL, inlineEmojiSize)
			md.AsyncInsertImage(ctx, row.Bounds[1], url, imgutil.WithRectRescale(inlineEmojiSize))
			// Insert the HTML.
			md.InsertInvisible(row.Bounds[1], customEmojiHTML(data))
		}
	default:
		log.Panicf("unknown data type %T", data)
	}

	return true
}

// requestAllMembers asynchronously fills up the local state with the given
// room's members.
func requestAllMembers(ctx context.Context, roomID matrix.RoomID) {
	client := gotktrix.FromContext(ctx)

	if err := client.RoomEnsureMembers(roomID); err != nil {
		app.Error(ctx, errors.Wrap(err, "failed to prefetch members"))
	}
}
