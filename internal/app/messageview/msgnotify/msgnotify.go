package msgnotify

import (
	"context"
	"strings"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/app"
	"github.com/diamondburned/gotktrix/internal/app/messageview/message/mauthor"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
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
		notify := client.NotifyMessage(message, gotktrix.NotifyMessage)
		if notify == 0 {
			return
		}

		const unreadIcon = app.NotificationIconName("unread-mail")
		icon := app.NotificationIcon(unreadIcon)

		avatar, _ := client.MemberAvatar(message.RoomID, message.Sender)
		if avatar != nil {
			avatarURL, _ := client.SquareThumbnail(*avatar, app.MaxNotificationIconSize, 1)
			icon = app.NotificationIconURL{
				Context:      ctx,
				URL:          avatarURL,
				FallbackIcon: unreadIcon,
			}
		}

		a := app.FromContext(ctx)
		a.SendNotification(app.Notification{
			ID:    app.HashNotificationID("new_message", client.UserID, message.RoomID),
			Title: mauthor.Name(client, message.RoomID, message.Sender),
			Body:  message.Body,
			Icon:  icon,
			Sound: app.MessageNotificationSound,
			Action: app.NotificationAction{
				ActionID: actionID,
				Argument: gtkutil.NewJSONVariant(OpenRoomCommand{
					UserID: client.UserID,
					RoomID: message.RoomID,
				}),
			},
		})
	})
}
