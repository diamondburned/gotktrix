package secret

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// Keyring is an implementation of a secret driver using the system's keyring
// driver.
type Keyring struct {
	id string
}

var ErrUnsupportedPlatform = keyring.ErrUnsupportedPlatform

var _ Driver = (*Keyring)(nil)

// KeyringDriver creates a new keyring driver.
func KeyringDriver(appID string) *Keyring {
	return &Keyring{appID}
}

// Set sets the key.
func (k *Keyring) Set(key string, value []byte) error {
	return keyring.Set(k.id, key, string(value))
}

// Get gets the key.
func (k *Keyring) Get(key string) ([]byte, error) {
	v, err := keyring.Get(k.id, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return []byte(v), nil
}
