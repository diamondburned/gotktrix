package mcontent

import (
	"context"
	"strconv"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/m"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/sortutil"
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
		margin:  0;
	}
	.mcontent-reaction {
		padding: 0;
		margin:  0;
	}
	.mcontent-reaction > button {
		padding: 0px 4px;
		margin:  0;
	}
`)

func newReactionBox() *reactionBox {
	rev := gtk.NewRevealer()
	rev.SetRevealChild(false)
	rev.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)

	return &reactionBox{
		Revealer: rev,
	}
}

func (r *reactionBox) Add(ctx context.Context, ev m.ReactionEvent) {
	if r.flowBox == nil {
		r.reactions = make(map[string]*reaction, 1)
		r.events = make(map[matrix.EventID]string, 1)

		f := gtk.NewFlowBox()
		f.SetRowSpacing(4)
		f.SetColumnSpacing(4)
		f.SetMaxChildrenPerLine(100)
		f.SetSelectionMode(gtk.SelectionNone)
		f.SetSortFunc(func(child1, child2 *gtk.FlowBoxChild) int {
			key1 := child1.Name()
			key2 := child2.Name()

			r1, ok1 := r.reactions[key1]
			r2, ok2 := r.reactions[key2]
			if !ok1 || !ok2 {
				return 0
			}

			if r1.amount != r2.amount {
				return intcmp(r1.amount, r2.amount)
			}

			return sortutil.CmpFold(key1, key2)
		})
		reactionsCSS(f)

		r.SetChild(f)
		r.SetRevealChild(true)
		r.flowBox = f
	} else {
		r, ok := r.reactions[ev.RelatesTo.Key]
		if ok {
			r.update(ctx, ev.SenderID, ev.EventID)
			return
		}
	}

	reaction := newReaction(ctx, ev)

	r.reactions[ev.RelatesTo.Key] = reaction
	r.flowBox.Insert(reaction, -1)
}

// Remove returns true if the given redaction event corresponds to a reaction.
func (r *reactionBox) Remove(ctx context.Context, red event.RoomRedactionEvent) bool {
	key, ok := r.events[red.EventID]
	if !ok {
		return false
	}

	delete(r.events, red.EventID)

	reaction, ok := r.reactions[key]
	if !ok {
		return true
	}

	reaction.update(ctx, red.SenderID, "")
	if reaction.amount > 0 {
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

	selfEv matrix.EventID
	amount int
}

func newReaction(ctx context.Context, ev m.ReactionEvent) *reaction {
	label := gtk.NewLabel(ev.RelatesTo.Key)
	label.SetSingleLineMode(true)
	label.SetEllipsize(pango.EllipsizeEnd)
	label.SetHExpand(true)
	label.SetMaxWidthChars(20)

	number := gtk.NewLabel("")
	number.SetMaxWidthChars(5)

	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.Append(label)
	box.Append(number)

	btn := gtk.NewToggleButton()
	btn.SetChild(box)
	btn.SetTooltipText(ev.RelatesTo.Key)

	client := gotktrix.FromContext(ctx).Offline()
	uID, _ := client.Whoami()
	if uID == ev.SenderID {
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
	}
	reaction.update(ctx, ev.SenderID, ev.EventID)

	// Use the first ever reaction event for this key as the event to send over.
	btn.Connect("clicked", func() { reaction.react(ctx, ev) })

	return &reaction
}

func (r *reaction) react(ctx context.Context, ev m.ReactionEvent) {
	client := gotktrix.FromContext(ctx)

	if r.selfEv != "" {
		evID := r.selfEv
		go func() {
			if err := client.Redact(ev.RoomID, evID, ""); err != nil {
				app.Error(ctx, errors.Wrap(err, "failed to unreact"))
				glib.IdleAdd(func() { r.btn.SetActive(true) })
				return
			}
		}()
	} else {
		go func() {
			client := gotktrix.FromContext(ctx)
			if err := client.SendRoomEvent(ev.RoomID, ev); err != nil {
				app.Error(ctx, errors.Wrap(err, "failed to react"))
			}
		}()
	}
}

func (r *reaction) update(
	ctx context.Context, sender matrix.UserID, addID matrix.EventID) {

	if addID != "" {
		r.amount++
	} else {
		r.amount--
	}

	r.number.SetLabel(strconv.Itoa(r.amount))

	client := gotktrix.FromContext(ctx).Offline()
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

func intcmp(i, j int) int {
	if i < j {
		return -1
	}
	if i == j {
		return 0
	}
	return 1
}
