package mcontent

import (
	"context"
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
	"github.com/diamondburned/gotktrix/internal/md"
	"github.com/diamondburned/gotktrix/internal/sortutil"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
	"github.com/pkg/errors"
)

type reactionBox struct {
	*gtk.Revealer
	flowBox   *gtk.FlowBox
	reactions map[string]*reaction
	events    map[matrix.EventID]string
}

var reactionsCSS = cssutil.Applier("mcontent-reactions", `
	.mcontent-reactions {
		padding: 0;
		margin-top:    4px;
		margin-bottom: 4px;
	}
	.mcontent-reaction {
		padding: 0;
		margin:  0;
	}
	.mcontent-reaction > button {
		padding: 0px 4px;
		margin:  0;
	}
	.mcontent-reaction > button {
		background-color: mix(@theme_fg_color, @theme_base_color, 0.85);
	}
	.mcontent-reaction > button:hover {
		background-color: mix(@theme_fg_color, @theme_base_color, 0.75);
	}
	.mcontent-reaction > button:checked {
		color: @theme_selected_fg_color;
		background-color: mix(mix(@theme_fg_color, @theme_base_color, 0.85), @theme_selected_bg_color, 0.4);
	}
	.mcontent-reaction > button:checked:hover {
		color: @theme_selected_fg_color;
		background-color: mix(mix(@theme_fg_color, @theme_base_color, 0.65), @theme_selected_bg_color, 0.4);
	}
`)

func newReactionBox() *reactionBox {
	r := reactionBox{}

	r.reactions = make(map[string]*reaction, 1)
	r.events = make(map[matrix.EventID]string, 1)

	r.flowBox = gtk.NewFlowBox()
	r.flowBox.SetRowSpacing(4)
	r.flowBox.SetColumnSpacing(4)
	r.flowBox.SetMaxChildrenPerLine(100)
	r.flowBox.SetSelectionMode(gtk.SelectionNone)
	r.flowBox.SetSortFunc(func(child1, child2 *gtk.FlowBoxChild) int {
		key1 := child1.Name()
		key2 := child2.Name()

		r1, ok1 := r.reactions[key1]
		r2, ok2 := r.reactions[key2]
		if !ok1 || !ok2 {
			return 0
		}

		if len(r1.people) != len(r2.people) {
			return intcmp(len(r1.people), len(r2.people))
		}

		return sortutil.CmpFold(key1, key2)
	})
	reactionsCSS(r.flowBox)

	r.Revealer = gtk.NewRevealer()
	r.Revealer.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	r.Revealer.SetChild(r.flowBox)

	// Show this up later.
	glib.IdleAdd(func() { r.Revealer.SetRevealChild(true) })

	return &r
}

func (r *reactionBox) Add(ctx context.Context, ev *m.ReactionEvent) {
	// Prevent registering the same event twice.
	_, registered := r.events[ev.ID]
	if registered {
		return
	}

	// Register this for Remove.
	r.events[ev.ID] = ev.RelatesTo.Key

	if reaction, ok := r.reactions[ev.RelatesTo.Key]; ok {
		reaction.update(ctx, ev.Sender, ev.ID)
		return
	}

	reaction := newReaction(ctx, ev)

	r.reactions[ev.RelatesTo.Key] = reaction
	r.flowBox.Insert(reaction, -1)
}

// Remove returns true if the given redaction event corresponds to a reaction.
func (r *reactionBox) Remove(ctx context.Context, red *event.RoomRedactionEvent) bool {
	key, ok := r.events[red.Redacts]
	if !ok {
		return false
	}

	delete(r.events, red.Redacts)

	reaction, ok := r.reactions[key]
	if !ok {
		return true
	}

	reaction.update(ctx, red.Sender, "")
	if len(reaction.people) > 0 {
		return true
	}

	r.flowBox.Remove(reaction)
	delete(r.reactions, key)
	r.SetRevealChild(len(r.reactions) > 0)
	return true
}

func (r *reactionBox) RemoveAll() {
	for id, reaction := range r.reactions {
		r.flowBox.Remove(reaction)
		delete(r.reactions, id)
	}

	r.SetRevealChild(false)
}

type reaction struct {
	*gtk.FlowBoxChild
	btn *gtk.ToggleButton

	box    *gtk.Box
	label  *gtk.Label
	number *gtk.Label

	roomID matrix.RoomID
	selfEv matrix.EventID
	people []reactedUser
}

type reactedUser struct {
	id   matrix.UserID
	name string
}

func newReaction(ctx context.Context, ev *m.ReactionEvent) *reaction {
	label := gtk.NewLabel(ev.RelatesTo.Key)
	label.SetSingleLineMode(true)
	label.SetEllipsize(pango.EllipsizeEnd)
	label.SetHExpand(true)
	label.SetMaxWidthChars(20)
	if !md.IsUnicodeEmoji(ev.RelatesTo.Key) {
		label.SetTooltipText(ev.RelatesTo.Key)
	}

	number := gtk.NewLabel("")
	number.SetMaxWidthChars(5)

	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.Append(label)
	box.Append(number)

	btn := gtk.NewToggleButton()
	btn.SetChild(box)

	client := gotktrix.FromContext(ctx).Offline()
	uID, _ := client.Whoami()
	if uID == ev.Sender {
		btn.SetActive(true)
	}

	child := gtk.NewFlowBoxChild()
	child.AddCSSClass("mcontent-reaction")
	child.SetName(ev.RelatesTo.Key)
	child.SetChild(btn)

	reaction := reaction{
		FlowBoxChild: child,

		btn:    btn,
		box:    box,
		label:  label,
		number: number,
		roomID: ev.RoomID,
	}
	reaction.update(ctx, ev.Sender, ev.ID)

	// Use the first ever reaction event for this key as the event to send over.
	btn.ConnectClicked(func() { reaction.react(ctx, ev) })

	return &reaction
}

func (r *reaction) react(ctx context.Context, ev *m.ReactionEvent) {
	// Mark insensitive. update() will change it back once an update arrives
	// from the server.
	r.SetSensitive(false)
	client := gotktrix.FromContext(ctx)

	evID := r.selfEv
	gtkutil.Async(ctx, func() func() {
		var err error

		unreact := evID != ""
		if unreact {
			err = client.Redact(ev.RoomID, evID, "")
		} else {
			err = client.SendRoomEvent(ev.RoomID, ev)
		}

		if err != nil {
			return func() {
				app.Error(ctx, errors.Wrap(err, "failed to react"))
				if unreact {
					// Failed to unreact, so change it back to active (reacted).
					r.btn.SetActive(true)
				}
			}
		}

		return nil
	})
}

func (r *reaction) update(
	ctx context.Context, sender matrix.UserID, addID matrix.EventID) {

	// Event arrived from server. Ensure the button is not disabled.
	r.SetSensitive(true)

	client := gotktrix.FromContext(ctx).Offline()

	if addID != "" {
		r.people = append(r.people, reactedUser{
			id: sender,
			name: mauthor.Markup(client, r.roomID, sender,
				mauthor.WithWidgetColor(),
				mauthor.WithMinimal(),
			),
		})
	} else {
		for i, user := range r.people {
			if user.id == sender {
				r.people = append(r.people[:i], r.people[i+1:]...)
				break
			}
		}
	}

	r.number.SetLabel(strconv.Itoa(len(r.people)))
	r.number.SetTooltipMarkup(reactedUserNames(ctx, r.people))

	uID, _ := client.Whoami()
	if uID == sender {
		r.btn.SetActive(true)

		if addID != "" {
			r.selfEv = addID
		} else {
			r.selfEv = ""
		}
	}
}

func reactedUserNames(ctx context.Context, people []reactedUser) string {
	const max = 25

	var hasMore bool
	n := len(people)

	if n > max {
		n = max
		hasMore = true
	}

	names := make([]string, n)
	for i := 0; i < n; i++ {
		names[i] = people[i].name
	}

	s := strings.Join(names, "\n")
	if hasMore {
		s += "\n" + locale.Sprintf(ctx, "and %d more", len(people)-max)
	}

	return s
}

func intcmp(i, j int) int {
	if i < j {
		return -1
	}
	if i == j {
		return 0
	}
	return 1
}
