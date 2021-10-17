// Package secret provides abstractions for storing data securely when possible.
package secret

import "github.com/pkg/errors"

// Service is the application ID of this package.
// const Service = "com.diamondburned.gotktrix.secrets"

// ErrNotFound is returned for unknown keys.
var ErrNotFound = errors.New("key not found")

// Driver is a basic getter-setter interface that describes a secret driver.
type Driver interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
}

// Service wraps multiple drivers to provide fallbacks.
type Service struct {
	drivers []Driver
}

// New creates a new service.
func New(drivers ...Driver) Service {
	return Service{drivers}
}

// Get gets the given key from the internal list of drivers. The first error is
// returned.
func (s Service) Get(k string) ([]byte, error) {
	var firstErr error

	for _, driver := range s.drivers {
		b, err := driver.Get(k)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		return b, nil
	}

	return nil, firstErr
}

// Set sets the given key and value into the internal list of drivers. The first
// successful driver is used, and only the first error is returned.
func (s Service) Set(k string, v []byte) error {
	var firstErr error

	for _, driver := range s.drivers {
		if err := driver.Set(k, v); err != nil {
			// Ignore not found errors, since other ones are more informative.
			if firstErr == nil && !errors.Is(err, ErrNotFound) {
				firstErr = err
			}
			continue
		}
		return nil
	}

	if firstErr == nil {
		// Use NotFound if there aren't any other errors.
		return ErrNotFound
	}

	return firstErr
}
