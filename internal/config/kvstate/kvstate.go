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

// Config is a single state configuration.
type Config struct {
	path  string
	store config.ConfigStore

	mut    sync.Mutex
	state  map[string]json.RawMessage
	loaded bool
}

// AcquireConfig creates a new Config instance.
func AcquireConfig(tails ...string) *Config {
	return acquireConfig(config.Path(tails...))
}

var registry = struct {
	sync.RWMutex
	cfgs map[string]*Config
}{
	cfgs: map[string]*Config{},
}

// acquireConfig creates a new state config.
func acquireConfig(path string) *Config {
	registry.RLock()
	c, ok := registry.cfgs[path]
	registry.RUnlock()

	if ok {
		return c
	}

	registry.Lock()
	defer registry.Unlock()

	c, ok = registry.cfgs[path]
	if ok {
		return c
	}

	c = &Config{path: path}
	c.store = config.NewConfigStore(c.snapshotFunc)

	registry.cfgs[path] = c
	return c
}

// Get gets the value of the key.
func (c *Config) Get(key string, dst interface{}) bool {
	c.mut.Lock()
	c.load()
	b, ok := c.state[key]
	c.mut.Unlock()

	if !ok {
		return false
	}

	if err := json.Unmarshal(b, dst); err != nil {
		log.Printf("cannot unmarshal %q into %T: %v", b, dst, err)
		return false
	}

	return true
}

// Exists returns true if key exists.
func (c *Config) Exists(key string) bool {
	c.mut.Lock()
	c.load()
	_, ok := c.state[key]
	c.mut.Unlock()

	return ok
}

// Set sets the value of the key. If val = nil, then the key is deleted.
func (c *Config) Set(key string, val interface{}) {
	var b []byte
	if val != nil {
		var err error

		b, err = json.Marshal(val)
		if err != nil {
			log.Panicf("cannot marshal %T: %v", val, err)
		}
	}

	c.mut.Lock()
	c.load()
	if val == nil {
		delete(c.state, key)
	} else {
		c.state[key] = b
	}
	c.mut.Unlock()

	c.store.Save()
}

// Delete calls Set(key, nil).
func (c *Config) Delete(key string) {
	c.Set(key, nil)
}

func (c *Config) load() {
	if c.loaded {
		return
	}
	c.loaded = true
	c.state = make(map[string]json.RawMessage)

	f, err := os.Open(c.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println("cannot open preference:", err)
		}
		return
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&c.state); err != nil {
		log.Printf("preference %q has invalid JSON: %v", c.path, err)
		return
	}
}

func (c *Config) snapshotFunc() func() {
	c.mut.Lock()
	defer c.mut.Unlock()

	if !c.loaded {
		log.Panicf("cannot snapshot unloaded config %q", c.path)
	}

	b, err := json.MarshalIndent(c.state, "", "\t")
	if err != nil {
		log.Panicln("cannot marshal kvstate.State:", err)
	}

	return func() {
		if err := config.WriteFile(c.path, b); err != nil {
			log.Println("cannot save kvstate:", err)
		}
	}
}
