package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/crypto/pbkdf2"
)

const (
	saltFile = ".salt"
	hashFile = ".hash"
)

// Reference: https://tutorialedge.net/golang/go-encrypt-decrypt-aes-tutorial/

// EncryptedFile is an implementation of a secret driver that encrypts the value
// stored using a generated salt. When created, EncryptedFileDriver should be
// used over SaltedFileDriver.
type EncryptedFile struct {
	path string // directory

	mu   sync.RWMutex
	pass string // use in the future for password
	aead cipher.AEAD
}

// SaltedFileDriver creates a new encrypted file driver with a generated
// passphrase.
func SaltedFileDriver(path string) *EncryptedFile {
	return &EncryptedFile{path: path}
}

// EncryptedFileDriver creates a new encrypted file driver with the given
// passphrase. The passphrase is hashed and compared with an existing one, or it
// will be used if there is none.
func EncryptedFileDriver(passphrase, path string) *EncryptedFile {
	return &EncryptedFile{path: path, pass: passphrase}
}

// mksalt makes the salt once or reads from a file if not.
func (s *EncryptedFile) getAEAD() (cipher.AEAD, error) {
	s.mu.RLock()
	aead := s.aead
	s.mu.RUnlock()

	if aead != nil {
		return aead, nil
	}

	// Reacquire to prevent race.
	s.mu.Lock()
	defer s.mu.Unlock()

	// Recheck to ensure that another routine didn't make the salt.
	if s.aead != nil {
		return s.aead, nil
	}

	pass, err := s.getPass()
	if err != nil {
		return nil, errors.Wrap(err, "failed to make/get salt")
	}

	c, err := aes.NewCipher(pass)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create AES cipher")
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create GCM cipherer")
	}

	s.pass = "" // no longer needed
	s.aead = gcm
	return gcm, nil
}

// hashAESKey hashes the given password and salt. This function takes 873ms on
// an Intel i5-8250U.
func hashAESKey(pass, salt []byte) []byte {
	return pbkdf2.Key(pass, salt, 2<<19, 32, sha512.New)
}

// getPass gets the PBKDF2-hashed key passphrase. Tihs function is safe from
// file bruteforcing, because all possible inputs are put through the hashing
// function before it is returned.
func (s *EncryptedFile) getPass() ([]byte, error) {
	if err := os.MkdirAll(s.path, 0600); err != nil {
		return nil, errors.Wrap(err, "failed to mkdir -p")
	}

	saltPath := filepath.Join(s.path, saltFile)

	salt, err := os.ReadFile(saltPath)
	if err != nil {
		// No existing salt. Generate and write one.
		salt = make([]byte, 64)

		_, err := rand.Read(salt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate salt")
		}

		if err := os.WriteFile(saltPath, salt, 0600); err != nil {
			return nil, errors.Wrap(err, "failed to write salt")
		}
	}

	// The user provided the password.
	if s.pass != "" {
		return hashAESKey([]byte(s.pass), salt), nil
	}

	// User did not provide a password. Try and generate our own locally.
	hashPath := filepath.Join(s.path, hashFile)

	// Try and read the existing hash.
	hash, err := os.ReadFile(hashPath)
	if err != nil {
		// No existing hash. Generate and write one.
		hash = make([]byte, 64)

		_, err := rand.Read(hash)
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate hash")
		}

		if err := os.WriteFile(hashPath, hash, 0600); err != nil {
			return nil, errors.Wrap(err, "failed to write hash")
		}
	}

	return hashAESKey(hash, salt), nil
}

func (s *EncryptedFile) Set(key string, value []byte) error {
	aead, err := s.getAEAD()
	if err != nil {
		return errors.Wrap(err, "failed to make salt")
	}

	nonce := make([]byte, aead.NonceSize())

	if _, err := rand.Read(nonce); err != nil {
		return errors.Wrap(err, "failed to read nonce")
	}

	file := base64.RawStdEncoding.EncodeToString([]byte(key))
	data := aead.Seal(nil, nonce, value, nil)

	if err := os.WriteFile(filepath.Join(s.path, file), data, 0600); err != nil {
		return errors.Wrap(err, "failed to write value to file")
	}

	return nil
}

func (s *EncryptedFile) Get(key string) ([]byte, error) {
	file := base64.RawStdEncoding.EncodeToString([]byte(key))

	b, err := os.ReadFile(filepath.Join(s.path, file))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("key does not exist")
		}
		return nil, errors.Wrap(err, "failed to get key")
	}

	aead, err := s.getAEAD()
	if err != nil {
		return nil, errors.Wrap(err, "failed to make salt")
	}

	if len(b) < aead.NonceSize() {
		return nil, errors.Wrap(err, "invalid file content")
	}

	value, err := aead.Open(nil, b[:aead.NonceSize()], b[aead.NonceSize():], nil)
	if err != nil {
		return nil, errors.Wrap(err, "decryption error")
	}

	return value, nil
}
