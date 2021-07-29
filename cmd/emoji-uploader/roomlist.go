package main

import (
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
)

type RoomList struct {
	*gtk.ListBox
	client *gotktrix.Client

	rooms   map[matrix.RoomID]Room
	current matrix.RoomID
}

type Room struct {
	name   string
	avatar string
}

func NewRoomList(client *gotktrix.Client) *RoomList {
	list := gtk.NewListBox()
	list.SetSizeRequest(250, -1)
	list.SetSelectionMode(gtk.SelectionBrowse)
	list.SetActivateOnSingleClick(true)
	list.SetPlaceholder(gtk.NewLabel("No rooms yet..."))

	roomList := RoomList{
		ListBox: list,
		client:  client,
	}

	list.SetSortFunc(func(i, j *gtk.ListBoxRow) int {
		return strings.Compare(i.Name(), j.Name())
	})

	list.Connect("row-activated", func(list *gtk.ListBox, row *gtk.ListBoxRow) {
		roomList.SetRoom(matrix.RoomID(row.Name()))
	})

	return &roomList
}

// SetIndex sets the active room to the given index.
func (l *RoomList) SetRoom(id matrix.RoomID) {
	l.current = id
	// room := l.rooms[id]

}
