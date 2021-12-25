package db

import (
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
)

// NodePath contains the full path to a node. It can be used as a lighter way to
// store nodes.
type NodePath [][]byte

// NewNodePath creates a new NodePath.
func NewNodePath(names ...string) NodePath {
	namesBytes := make([][]byte, len(names))
	for i := range names {
		namesBytes[i] = []byte(names[i])
	}

	return namesBytes
}

// Tail creates a copy of NodePath with the given tail.
func (p NodePath) Tail(tails ...string) NodePath {
	namesBytes := make([][]byte, len(p), len(p)+len(tails))
	copy(namesBytes, p)
	for i := range tails {
		namesBytes = append(namesBytes, []byte(tails[i]))
	}
	return namesBytes
}

// NodePath traverses the given transaction and returns the bucket. If any of
// the buckets don't exist, then a new one is created, unless the transaction is
// read-only.
//
// If NodePath is empty, then a Bucket with a nil byte is returned.
func (p NodePath) Bucket(tx *bbolt.Tx) (*bbolt.Bucket, error) {
	return p.bucket(tx, false)
}

// BucketExists traverses and returns true if the bucket exists.
func (p NodePath) BucketExists(tx *bbolt.Tx) (*bbolt.Bucket, bool) {
	b, err := p.bucket(tx, true)
	return b, err == nil
}

func (p NodePath) bucket(tx *bbolt.Tx, ro bool) (*bbolt.Bucket, error) {
	if len(p) == 0 {
		b, err := getBucketRoot(tx, nil, ro)
		if err != nil {
			return nil, BucketError{err}
		}
		return b, nil
	}

	b, err := getBucketRoot(tx, p[0], ro)
	if err != nil {
		return nil, BucketError{err}
	}

	for _, path := range p[1:] {
		if b, err = getBucket(b, path, ro); err != nil {
			return nil, BucketError{err}
		}
	}

	return b, nil
}

func getBucketRoot(tx *bbolt.Tx, k []byte, ro bool) (*bbolt.Bucket, error) {
	if tx.Writable() && !ro {
		return tx.CreateBucketIfNotExists(k)
	}
	if b := tx.Bucket(k); b != nil {
		return b, nil
	}
	return nil, bbolt.ErrBucketNotFound
}

func getBucket(b *bbolt.Bucket, k []byte, ro bool) (*bbolt.Bucket, error) {
	if b.Writable() && !ro {
		return b.CreateBucketIfNotExists(k)
	}
	if b := b.Bucket(k); b != nil {
		return b, nil
	}
	return nil, bbolt.ErrBucketNotFound
}

type KV struct {
	Marshaler
	db *bbolt.DB
}

func NewKVFile(path string) (*KV, error) {
	// Ensure that the parent directory are all created.
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, errors.Wrap(err, "failed to create db directory")
	}

	db, err := bbolt.Open(path, os.ModePerm, &bbolt.Options{
		Timeout:      10 * time.Second,
		FreelistType: bbolt.FreelistMapType,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to open db")
	}

	return &KV{
		Marshaler: JSONMarshaler,
		db:        db,
	}, nil
}

// DropPrefix drops the whole given prefix.
func (kv *KV) DropPrefix(path NodePath) error {
	return kv.db.Update(func(tx *bbolt.Tx) error {
		return dropBucketPrefix(tx, path)
	})
}

func dropBucketPrefix(tx *bbolt.Tx, path NodePath) error {
	if len(path) == 0 {
		return errors.New("cannot delete whole database")
	}

	if len(path) == 1 {
		return wrapBucketDeleteErr(tx.DeleteBucket(path[0]))
	}

	// Slice off the last bucket name.
	b, err := path[:len(path)-1].Bucket(tx)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			// Treat as already deleted.
			return nil
		}
		return err
	}

	return wrapBucketDeleteErr(b.DeleteBucket(path[len(path)-1]))
}

func wrapBucketDeleteErr(err error) error {
	if errors.Is(err, bbolt.ErrBucketNotFound) {
		// No need to wipe.
		return nil
	}
	return err
}

// NodeFromPath creates a new Node from path.
func (kv *KV) NodeFromPath(path NodePath) Node {
	return Node{
		kv:   kv,
		path: path,
	}
}

// Node creates a new Node from the given names joined as paths.
func (kv *KV) Node(names ...string) Node {
	if len(names) == 0 {
		panic("Node name can't be empty")
	}

	return kv.NodeFromPath(NewNodePath(names...))
}

// Close closes the database.
func (kv *KV) Close() error {
	kv.db.Sync()
	return kv.db.Close()
}
