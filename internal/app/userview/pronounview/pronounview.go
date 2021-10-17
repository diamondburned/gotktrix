package pronounview

// UpdateDialog provides a dialog to change a user's pronouns. The dialog will
// provide a small popover containing an entry to add new pronouns and a radio
// selector to choose the preferred one.
//
// When the user changes the pronouns, the user event will be sent and the room
// events will be broadcasted to all rooms.
type UpdateDialog struct {
}
