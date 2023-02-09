package gotktrix

import "maunium.net/go/mautrix"

type syncer struct {
	mautrix.DefaultSyncer
}

func newSyncer() *syncer {
	return &syncer{
		DefaultSyncer: *mautrix.NewDefaultSyncer(),
	}
}

func (s *syncer) GetFilterJSON(uID id.UserID) *mautrix.Filter {
	// Filter: event.GlobalFilter{
	// 	Room: event.RoomFilter{
	// 		State: event.StateFilter{
	// 			LazyLoadMembers:         true,
	// 			IncludeRedundantMembers: true,
	// 		},
	// 		Timeline: event.RoomEventFilter{
	// 			Limit:           TimelimeLimit,
	// 			LazyLoadMembers: true,
	// 		},
	// 	},
	// },
	return &mautrix.Filter{
		Room: mautrix.RoomFilter{
			State: mautrix.StateFilter{
				LazyLoadMembers:         true,
				IncludeRedundantMembers: true,
			},
	}
}
