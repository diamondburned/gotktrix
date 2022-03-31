// Package osutil provides helper functions around package os.
package osutil

import (
	"io"
	"os"

	"github.com/pkg/errors"
)

// TempFile is a temporary file.
type TempFile struct {
	os.File
}

type copyError struct {
	err error
}

func (err copyError) Error() string {
	return err.err.Error()
}

func (err copyError) Unwrap() error {
	return err.err
}

// IsCorrupted returns true if the given error has occured during copying. If
// this is true, then the user must notify the user and discard the reader.
func IsCorrupted(err error) bool {
	var cpyErr *copyError
	return errors.As(err, &cpyErr)
}

// Consume consumes the given io.Reader and returns a temporary file that will
// be deleted when it is closed.
func Consume(r io.Reader) (*TempFile, error) {
	f, err := Mktemp("")
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return nil, copyError{err}
	}

	// Seek the cursor back to start of file.
	if err := f.Rewind(); err != nil {
		f.Close()
		return nil, copyError{err}
	}

	return f, nil
}

// Mktemp creates a new temp file in a predefined temporary directory. Pattern
// is optional.
func Mktemp(pattern string) (*TempFile, error) {
	tempDir := os.TempDir()

	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		return nil, errors.Wrap(err, "failed to make tempDir")
	}

	f, err := os.CreateTemp(tempDir, pattern)
	if err != nil {
		return nil, err
	}

	return &TempFile{*f}, nil
}

// Close closes the file and removes it.
func (t *TempFile) Close() error {
	err1 := t.File.Close()
	err2 := os.Remove(t.Name())

	if err1 != nil && !errors.Is(err1, os.ErrClosed) {
		return err1
	}
	return err2
}

// Open opens the current temporary file. The new file will not remove the
// temporary file when it is closed. This is useful for creating simultaneous
// readers on the same file.
func (t *TempFile) Open() (*os.File, error) {
	f, err := os.Open(t.Name())
	if err != nil {
		return nil, errors.Wrap(err, "failed to open same file")
	}

	o, err := t.Seek(0, io.SeekCurrent)
	if err == nil {
		f.Seek(o, io.SeekStart)
	}

	return f, nil
}

// Rewind resets the file reader cursor to the start position. This is a
// convenient function around Seek.
func (t *TempFile) Rewind() error {
	_, err := t.Seek(0, io.SeekStart)
	return err
}
