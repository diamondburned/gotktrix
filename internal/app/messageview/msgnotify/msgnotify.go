package msgnotify

import (
	"context"
	"strings"

	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/notify"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotrix/event"
	"github.com/diamondburned/gotrix/matrix"
)

// OpenRoomCommand is the command structure for the open room action.
type OpenRoomCommand struct {
	UserID matrix.UserID `json:"user_id"`
	RoomID matrix.RoomID `json:"room_id"`
}

// StartNotify starts notifying the user for any new messages that mentions the
// user. A stop callback is returned. actionID must be application-scoped and
// therefore have the "app." prefix.
func StartNotify(ctx context.Context, actionID string) (stop func()) {
	if !strings.HasPrefix(actionID, "app.") {
		panic("actionID does not have the app prefix")
	}

	client := gotktrix.FromContext(ctx)
	return client.SubscribeAllTimeline(func(ev event.RoomEvent) {
		message, ok := ev.(*event.RoomMessageEvent)
		if !ok {
			return
		}

		// TODO: NotifySoundMessage?
		action := client.NotifyMessage(message, gotktrix.NotifyMessage)
		if action == 0 {
			return
		}

		unreadIcon := notify.IconName("unread-mail")
		icon := notify.Icon(unreadIcon)

		avatar, _ := client.MemberAvatar(message.RoomID, message.Sender)
		if avatar != nil {
			avatarURL, _ := client.SquareThumbnail(*avatar, notify.MaxIconSize, 1)
			icon = notify.IconURL(ctx, avatarURL, unreadIcon)
		}

		notification := notify.Notification{
			ID:    notify.HashID("new_message", client.UserID, message.RoomID),
			Title: mauthor.Name(client, message.RoomID, message.Sender),
			Body:  message.Body,
			Icon:  icon,
			Sound: notify.MessageSound,
			Action: notify.Action{
				ActionID: actionID,
				Argument: gtkutil.NewJSONVariant(OpenRoomCommand{
					UserID: client.UserID,
					RoomID: message.RoomID,
				}),
			},
		}

		a := app.FromContext(ctx)
		notification.Send(a)
	})
}
