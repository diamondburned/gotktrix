package roomsort

import (
	"sort"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
)

// SortMode describes the possible ways to sort a room.
type SortMode uint8

const (
	SortAlphabetically SortMode = iota
	SortActivity
)

// Sorter is a sorter interface that implements sort.Interface. It sorts rooms.
type Sorter struct {
	Comparer

	// SwapFn is an additional function that the user could use to synchronize
	// sorting with another data structure, such as a ListBox.
	SwapFn func(i, j int)
	Rooms  []matrix.RoomID
}

// SortedRooms returns the list of rooms sorted.
func SortedRooms(client *gotktrix.Client, mode SortMode) ([]matrix.RoomID, error) {
	rooms, err := client.Rooms()
	if err != nil {
		return nil, err
	}

	Sort(client, rooms, mode)
	return rooms, nil
}

// Sort sorts a room using a Sorter. It's a convenient function.
func Sort(client *gotktrix.Client, rooms []matrix.RoomID, mode SortMode) {
	sorter := NewSorter(client, rooms, mode)
	sorter.Sort()
}

// NewSorter creates a new sorter.
func NewSorter(client *gotktrix.Client, rooms []matrix.RoomID, mode SortMode) *Sorter {
	sorter := &Sorter{
		Comparer: *NewComparer(client, mode),
		Rooms:    rooms,
	}
	sorter.Comparer.List = sorter
	return sorter
}

// Add adds the given room IDs and resort the whole list.
func (sorter *Sorter) Add(ids ...matrix.RoomID) {
	sorter.Rooms = append(sorter.Rooms, ids...)
	sorter.Sort()
}

// Sort sorts the sorter. The internal room name/timestamp cache is invalidated
// before sorting; see InvalidateRoomCache for more information.
func (sorter *Sorter) Sort() {
	sorter.InvalidateRoomCache()
	sort.Sort(sorter)
}

// FirstID returns the first room ID if any.
func (sorter *Sorter) FirstID() matrix.RoomID {
	if len(sorter.Rooms) > 0 {
		return sorter.Rooms[0]
	}
	return ""
}

// LastID returns the last room ID if any.
func (sorter *Sorter) LastID() matrix.RoomID {
	if len(sorter.Rooms) > 0 {
		return sorter.Rooms[len(sorter.Rooms)-1]
	}
	return ""
}

// Len returns the length of sorter.Rooms.
func (sorter *Sorter) Len() int { return len(sorter.Rooms) }

// Swap swaps the entries inside Rooms.
func (sorter *Sorter) Swap(i, j int) {
	sorter.Rooms[i], sorter.Rooms[j] = sorter.Rooms[j], sorter.Rooms[i]

	if sorter.SwapFn != nil {
		sorter.SwapFn(i, j)
	}
}

// Less returns true if the room indexed [i] should be before [j].
func (sorter *Sorter) Less(i, j int) bool {
	return sorter.Comparer.Less(sorter.Rooms[i], sorter.Rooms[j])
}
