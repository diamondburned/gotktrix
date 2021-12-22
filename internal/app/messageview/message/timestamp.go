package message

import (
	"context"
	"fmt"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/locale"
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
	l.SetTooltipText(ts.Format(time.StampMilli))
	timestampCSS(l)

	return &timestamp{l, ctx, ts, long}
}

func (t *timestamp) setEdited(editedTs time.Time) {
	t.SetTooltipText(fmt.Sprintf(
		"%s "+locale.S(t.ctx, "(edited %s)"),
		absTime(t.time),
		absTime(editedTs),
	))
	if t.long {
		t.SetText(fmt.Sprintf(
			"%s "+locale.S(t.ctx, "(edited)"),
			locale.TimeAgo(t.ctx, t.time),
			locale.TimeAgo(t.ctx, editedTs),
		))
	}
}

func absTime(t time.Time) string {
	return t.Format("Jan 2 15:04:05.000000")
}
