package message

import (
	"context"
	"fmt"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

var timestampCSS = cssutil.Applier("message-timestamp", `
	.message-timestamp {
		font-size: 0.80em;
		color: alpha(@theme_fg_color, 0.55);
	}
`)

type timestamp struct {
	*gtk.Label
	ctx  context.Context
	time time.Time
	long bool
}

// newTimestamp creates a new timestamp label. If long is true, then the label
// timestamp is long.
func newTimestamp(ctx context.Context, ts time.Time, long bool) *timestamp {
	var t string
	if long {
		t = locale.TimeAgo(ctx, ts)
	} else {
		t = locale.Time(ts, false)
	}

	l := gtk.NewLabel(t)
	l.SetTooltipText(locale.Time(ts, true))
	l.SetEllipsize(pango.EllipsizeMiddle)
	timestampCSS(l)

	return &timestamp{l, ctx, ts, long}
}

func (t *timestamp) setEdited(editedTs time.Time) {
	t.SetTooltipText(locale.Sprintf(t.ctx,
		"%s (edited %s)",
		locale.Time(t.time, true),
		locale.Time(editedTs, true),
	))
	if t.long {
		t.SetText(fmt.Sprintf(
			"%s "+locale.S(t.ctx, "(edited)"),
			locale.TimeAgo(t.ctx, t.time),
		))
	}
}
