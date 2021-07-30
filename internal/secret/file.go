package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/crypto/pbkdf2"
)

const (
	saltFile = ".salt" // salt input
	saltSize = 64

	hashFile   = ".hash" // final hash output for comparison
	hashRounds = 2 << 19
)

// Reference: https://tutorialedge.net/golang/go-encrypt-decrypt-aes-tutorial/

// EncryptedFile is an implementation of a secret driver that encrypts the value
// stored using a generated salt. When created, EncryptedFileDriver should be
// used over SaltedFileDriver.
type EncryptedFile struct {
	path string // directory

	mu   sync.RWMutex
	aead cipher.AEAD

	pass string
	enc  bool
}

// SaltedFileDriver creates a new encrypted file driver with a generated
// passphrase. The .salt file is solely used as the hashing input, so the
// algorithm will trip without it. One way to completely lock out accounts
// encrypted with it is to move the file somewhere else.
func SaltedFileDriver(path string) *EncryptedFile {
	return &EncryptedFile{path: path}
}

// EncryptedFileDriver creates a new encrypted file driver with the given
// passphrase. The passphrase is hashed and compared with an existing one, or it
// will be used if there is none.
func EncryptedFileDriver(passphrase, path string) *EncryptedFile {
	return &EncryptedFile{path: path, pass: passphrase, enc: true}
}

// PathIsEncrypted returns true if the given path is encrypted. It is the
// caller's responsibility to use SaltedFileDriver or EncryptedFileDriver on the
// same path.
//
// In some cases, false will be returned if the status of encryption cannot be
// determined. In this case, when EncryptedFileDriver is used, storing will be
// errored out.
func PathIsEncrypted(path string) bool {
	hashPath := filepath.Join(path, hashFile)

	f, err := os.Stat(hashPath)
	if err != nil || f.IsDir() {
		return false
	}

	return true
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
	return pbkdf2.Key(pass, salt, hashRounds, 32, sha512.New)
}

// ErrIncorrectPassword is returned if the provided user password does not match
// what is on disk.
var ErrIncorrectPassword = errors.New("incorrect password")

// getPass gets the PBKDF2-hashed key passphrase. Tihs function is safe from
// file bruteforcing, because all possible inputs are put through the hashing
// function before it is returned.
func (s *EncryptedFile) getPass() ([]byte, error) {
	if err := os.MkdirAll(s.path, 0700); err != nil {
		return nil, errors.Wrap(err, "failed to mkdir -p")
	}

	saltPath := filepath.Join(s.path, saltFile)
	hashPath := filepath.Join(s.path, hashFile)

	salt, err := os.ReadFile(saltPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to read old salt")
	}

	hash, err := os.ReadFile(hashPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "failed to read old hash")
	}

	if hash != nil && salt == nil {
		// Old hash exists, but not the salt. We can't decrypt this.
		return nil, errors.New("missing salt file")
	}

	if salt == nil {
		// Hash does not exist, and we have no existing salt. Generate and write
		// one.
		salt = make([]byte, saltSize)

		_, err := rand.Read(salt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate salt")
		}

		if err := os.WriteFile(saltPath, salt, 0600); err != nil {
			return nil, errors.Wrap(err, "failed to write salt")
		}
	}

	password := salt
	if s.enc {
		// User provided a password. Use that instead.
		password = []byte(s.pass)
	}

	userHash := hashAESKey(password, salt)

	if hash != nil {
		// User have already encrypted in the past. Compare this password with
		// the old one.
		if subtle.ConstantTimeCompare(userHash, hash) == 1 {
			return userHash, nil
		}
		return nil, ErrIncorrectPassword
	}

	// User have not encrypted before. Save the hash file.
	if err := os.WriteFile(hashPath, userHash, 0600); err != nil {
		return nil, errors.Wrap(err, "failed to save hash")
	}

	return userHash, nil
}

func (s *EncryptedFile) Set(key string, value []byte) error {
	aead, err := s.getAEAD()
	if err != nil {
		return errors.Wrap(err, "failed to get cipher")
	}

	nonce := make([]byte, aead.NonceSize())

	if _, err := rand.Read(nonce); err != nil {
		return errors.Wrap(err, "failed to read nonce")
	}

	// Append the encrypted data into the nonce for this key.
	data := aead.Seal(nonce, nonce, value, nil)
	file := base64.RawStdEncoding.EncodeToString([]byte(key))

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
			return nil, ErrNotFound
		}
		return nil, errors.Wrap(err, "failed to get key")
	}

	aead, err := s.getAEAD()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cipher")
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
