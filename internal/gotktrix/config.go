package gotktrix

import "path/filepath"

// ConfigPather is an interface describing any instance that can generate a
// ConfigPath for gotktrix. Realistically, app.Application implements this.
type ConfigPather interface {
	ConfigPath(tails ...string) string
}

type constConfigPath string

func (p constConfigPath) ConfigPath(tails ...string) string {
	v := append([]string{string(p)}, tails...)
	return filepath.Join(v...)
}
