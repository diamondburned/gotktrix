package config

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// SaverWidget describes anything that can begin showing to the user that it's
// saving and done saving.
type SaverWidget interface {
	gtk.Widgetter
	SaveBegin()
	SaveEnd()
}

// ConfigStore implements store debouncing for a widget.
type ConfigStore struct {
	Widget  SaverWidget
	Minimum time.Duration

	snapshot    func() (save func())
	isSaving    bool
	needsSaving bool
}

// NewConfigStore creates a new ConfigStore instance. save is called in a
// goroutine.
func NewConfigStore(snapshot func() (save func())) ConfigStore {
	return ConfigStore{snapshot: snapshot}
}

func (s *ConfigStore) Save() {
	s.needsSaving = true
	s.save()
}

func (s *ConfigStore) save() {
	if s.isSaving {
		return
	}
	s.isSaving = true

	if s.Widget != nil {
		s.Widget.SaveBegin()
	}

	min := s.Minimum
	save := s.snapshot()

	go func() {
		var ch <-chan time.Time
		if min > 0 {
			ch = time.After(min)
		}

		save()

		if ch != nil {
			<-ch
		}

		glib.IdleAdd(func() {
			if s.Widget != nil {
				s.Widget.SaveEnd()
			}

			s.isSaving = false

			if s.needsSaving {
				s.needsSaving = false
				s.save()
			}
		})
	}()
}
