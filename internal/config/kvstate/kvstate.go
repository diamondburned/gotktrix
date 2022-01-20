// Package kvstate provides a small key-value configuration database for use in
// various components to store state.
package kvstate

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/diamondburned/gotktrix/internal/config"
)

// State is a single state configuration.
type State struct {
	path  string
	store config.ConfigStore

	mut    sync.Mutex
	state  map[string]json.RawMessage
	loaded bool
}

// NewConfigState is a convenient function around NewState.
func NewConfigState(tails ...string) *State {
	tails = append([]string{"app-state"}, tails...)
	return newState(config.Path(tails...))
}

// newState creates a new state config.
func newState(path string) *State {
	s := State{path: path}
	s.store = config.NewConfigStore(s.snapshotFunc)
	return &s
}

// Get gets the value of the key.
func (s *State) Get(key string, dst interface{}) {
	s.mut.Lock()
	s.load()
	b := s.state[key]
	s.mut.Unlock()

	if err := json.Unmarshal(b, dst); err != nil {
		log.Panicf("cannot unmarshal %q into %T: %v", b, dst, err)
	}
}

// Set sets the value of the key.
func (s *State) Set(key string, val interface{}) {
	b, err := json.Marshal(val)
	if err != nil {
		log.Panicf("cannot marshal %T: %v", val, err)
	}

	s.mut.Lock()
	s.load()
	s.state[key] = b
	s.mut.Unlock()
}

func (s *State) load() {
	if s.loaded {
		return
	}
	s.loaded = true

	f, err := os.Open(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println("cannot open preference:", err)
		}
		return
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&s.state); err != nil {
		log.Printf("preference %q has invalid JSON: %v", s.path, err)
		return
	}
}

func (s *State) snapshotFunc() func() {
	b, err := json.MarshalIndent(s.state, "", "\t")
	if err != nil {
		log.Panicln("cannot marshal kvstate.State:", err)
	}

	return func() {
		if err := config.WriteFile(s.path, b); err != nil {
			log.Println("cannot save kvstate")
		}
	}
}
