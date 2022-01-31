package app

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotktrix/internal/config/prefs"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

// MaxNotificationIconSize is the maximum size of the notification icon to give to
// the gio.Icon.
const MaxNotificationIconSize = 64

// NotificationIcon is a type for a notification icon.
type NotificationIcon interface {
	async() bool
	icon() gio.Iconner // can return nil
}

// NotificationIconName is a notification icon that follows the system icon
// theme.
type NotificationIconName string

func (n NotificationIconName) async() bool { return false }

func (n NotificationIconName) icon() gio.Iconner {
	if n == "" {
		return nil
	}
	return gio.NewThemedIcon(string(n))
}

// NotificationIconURL is a notification icon that is an image fetched online.
// The image is fetched using imgutil.GETPixbuf.
type NotificationIconURL struct {
	Context      context.Context
	URL          string // if empty, will use fallback
	FallbackIcon NotificationIconName
}

func (n NotificationIconURL) async() bool {
	return n.URL != ""
}

func (n NotificationIconURL) icon() gio.Iconner {
	if n.URL == "" {
		return n.FallbackIcon.icon()
	}

	p, err := imgutil.GETPixbuf(
		n.Context, n.URL,
		imgutil.WithRescale(MaxNotificationIconSize, MaxNotificationIconSize))
	if err != nil {
		log.Println("cannot GET notification icon URL:", err)
		return n.FallbackIcon.icon()
	}

	b, err := p.SaveToBufferv("png", []string{"compression"}, []string{"0"})
	if err != nil {
		log.Println("cannot save notification icon URL as PNG:", err)
		return n.FallbackIcon.icon()
	}

	return gio.NewBytesIcon(glib.NewBytesWithGo(b))
}

// NotificationSound is a type for a notification sound.
type NotificationSound string

// Known notification sound constants.
const (
	NoNotificationSound      NotificationSound = ""
	BellNotificationSound    NotificationSound = "bell"
	MessageNotificationSound NotificationSound = "message"
)

// NotificationID is a type for a notification ID. It exists so convenient
// hashing functions can exist. If the ID is empty, then GTK will internally
// generate a new one. There's no way to recall/change the notification then.
type NotificationID string

// HashNotificationID are created from hashing the given inputs. This is useful
// for generating short notification IDs that are uniquely determined by the
// inputs.
func HashNotificationID(keys ...interface{}) NotificationID {
	// We're not actually hashing any of this. We don't need to.
	var b strings.Builder
	for _, key := range keys {
		fmt.Fprint(&b, key)
		b.WriteByte(';')
	}
	return NotificationID(b.String())
}

// NotificationAction is an action of a notification.
type NotificationAction struct {
	ActionID string
	Argument *glib.Variant
}

// Notification is a data structure for a notification. A GNotification object
// is created from this type.
type Notification struct {
	ID    NotificationID
	Title string // required
	Body  string
	// Icon is the notification icon. If it's nil, then the application's icon
	// is used.
	Icon NotificationIcon
	// Action is the action to activate if the notification is clicked.
	Action NotificationAction
	// Priority is the priority of the notification.
	Priority gio.NotificationPriority
	// Sound, if true, will ring a sound. If it's an empty string, then no sound
	// is played.
	Sound NotificationSound
}

// async returns true if the notification must be constructed within a
// goroutine.
func (n *Notification) async() bool {
	return n.Sound != "" || (n.Icon != nil && n.Icon.async())
}

func (n *Notification) asGio() *gio.Notification {
	if n.Title == "" {
		panic("notification missing Title")
	}

	notification := gio.NewNotification(n.Title)

	if n.Body != "" {
		notification.SetBody(n.Body)
	}

	if n.Priority != 0 {
		notification.SetPriority(n.Priority)
	}

	if n.Icon != nil {
		if icon := n.Icon.icon(); icon != nil {
			notification.SetIcon(icon)
		}
	}

	if n.Action != (NotificationAction{}) {
		notification.SetDefaultActionAndTarget(n.Action.ActionID, n.Action.Argument)
	}

	return notification
}

var showNotification = prefs.NewBool(true, prefs.PropMeta{
	Name:    "Show Notifications",
	Section: "Application",
	Description: "Show a notification for messages that mention the user. " +
		"No notifications are triggered if the user is focused on the window",
})

var playNotificationSound = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Play Notification Sound",
	Section:     "Application",
	Description: "Play a sound every time a notification pops up.",
})

func init() {
	prefs.Order(showNotification, playNotificationSound)
}

func (n *Notification) playSound() {
	if n.Sound == NoNotificationSound || !playNotificationSound.Value() {
		return
	}

	// Try with canberra-gtk-theme.
	canberra := exec.Command("canberra-gtk-play", "--id", string(n.Sound))
	if err := canberra.Run(); err != nil {
		log.Println("notifying using beep() because:", err)

		disp := gdk.DisplayGetDefault()
		disp.Beep()
	}
}

func (n *Notification) send(app *gio.Application) {
	if !showNotification.Value() {
		return
	}

	if !n.async() {
		app.SendNotification(string(n.ID), n.asGio())
		return
	}

	go func() {
		app.SendNotification(string(n.ID), n.asGio())
		n.playSound()
	}()
}
