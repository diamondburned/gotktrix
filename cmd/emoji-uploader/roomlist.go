package main

import (
	"context"

	"github.com/chanbakjsd/gotrix/event"
	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix/events/roomsort"
	"github.com/diamondburned/gotktrix/internal/gtkutil/cssutil"
	"github.com/diamondburned/gotktrix/internal/gtkutil/imgutil"
)

type RoomList struct {
	*gtk.ListBox
	client *gotktrix.Client

	onRoom func(matrix.RoomID)

	rooms   map[matrix.RoomID]room
	current matrix.RoomID
}

const avatarSize = 32

type room struct {
	name   *gtk.Label
	avatar *adw.Avatar
}

var avatarCSS = cssutil.Applier("emojiuploader-avatar", `
	.emojiuploader-avatar {
		padding: 2px 4px;
	}
`)

func NewRoomList(client *gotktrix.Client) *RoomList {
	list := gtk.NewListBox()
	list.SetSizeRequest(200, -1)
	list.SetSelectionMode(gtk.SelectionBrowse)
	list.SetActivateOnSingleClick(true)
	list.SetPlaceholder(gtk.NewLabel("No rooms yet..."))

	roomList := RoomList{
		ListBox: list,
		client:  client,
	}

	comparer := roomsort.NewComparer(client, roomsort.SortAlphabetically)

	list.SetSortFunc(func(i, j *gtk.ListBoxRow) int {
		iID := matrix.RoomID(i.Name())
		jID := matrix.RoomID(j.Name())
		if iID == jID {
			return 0
		}
		if comparer.Less(iID, jID) {
			return -1
		}
		return 1
	})

	list.Connect("row-activated", func(list *gtk.ListBox, row *gtk.ListBoxRow) {
		roomList.setRoom(matrix.RoomID(row.Name()))
	})

	return &roomList
}

// OnRoom sets the function to be called when a room is selected.
func (l *RoomList) OnRoom(f func(matrix.RoomID)) {
	l.onRoom = f
}

// AddRooms adds the rooms with the given IDs.
func (l *RoomList) AddRooms(roomIDs []matrix.RoomID) {
	state := l.client.WithContext(gotktrix.Cancelled())

	for _, roomID := range roomIDs {
		nameLabel := gtk.NewLabel(string(roomID))
		nameLabel.SetXAlign(0)
		nameLabel.SetEllipsize(pango.EllipsizeMiddle)
		nameLabel.SetHExpand(true)

		adwAvatar := adw.NewAvatar(avatarSize, "#", true)
		avatarCSS(&adwAvatar.Widget)

		// TODO: async fetch
		if name, err := state.RoomName(roomID); err == nil {
			nameLabel.SetLabel(name)
		}

		ev, err := state.RoomState(roomID, event.TypeRoomAvatar, "")
		if err == nil {
			avatarEv := ev.(event.RoomAvatarEvent)
			url, _ := state.SquareThumbnail(avatarEv.URL, avatarSize)
			imgutil.AsyncGET(context.TODO(), url, adwAvatar.SetCustomImage)
		}

		box := gtk.NewBox(gtk.OrientationHorizontal, 0)
		box.Append(&adwAvatar.Widget)
		box.Append(nameLabel)

		row := gtk.NewListBoxRow()
		row.SetChild(box)
		row.SetName(string(roomID))

		l.ListBox.Insert(row, -1)
	}
}

func (l *RoomList) setRoom(id matrix.RoomID) {
	l.current = id
	if l.onRoom != nil {
		l.onRoom(id)
	}
}
