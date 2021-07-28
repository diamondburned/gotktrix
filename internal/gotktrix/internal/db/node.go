package db

import (
	"bytes"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"
)

var ErrKeyNotFound = errors.New("Key not found in database")

// Keys joins the given keys with the delimiter inbetween. This is an
// alternative over calling .Node().
func Keys(keys ...string) string {
	if len(keys) == 1 {
		return keys[0]
	}
	if len(keys) == 2 && keys[1] == "" {
		return keys[1] + delimiter
	}

	return strings.Join(keys, delimiter)
}

func convertKey(prefix [][]byte, key string) []byte {
	if key == "" {
		return bytes.Join(prefix, []byte(delimiter))
	}

	return bytes.Join(append(prefix, []byte(key)), []byte(delimiter))
}

func convertPrefix(prefix [][]byte) []byte {
	return bytes.Join(prefix, []byte(delimiter))
}

func appendString(bts []byte, str string) []byte {
	return append(bts, []byte(str)...)
}

func wrapErr(str string, err error) error {
	if err == nil {
		return nil
	}

	if !errors.Is(err, badger.ErrKeyNotFound) {
		log.Println("Unexpected db error:", err)
	} else {
		return ErrKeyNotFound
	}

	return errors.Wrap(err, str)
}

func iterKeyOnlyOpts(prefix []byte) badger.IteratorOptions {
	o := badger.DefaultIteratorOptions
	o.PrefetchValues = false
	o.Prefix = prefix
	return o
}

func iterOpts(prefix []byte) badger.IteratorOptions {
	o := badger.DefaultIteratorOptions
	o.PrefetchValues = true
	o.PrefetchSize = 1
	o.Prefix = prefix
	return o
}

type Node struct {
	prefixes [][]byte
	kvdb     KV
}

// Prefix returns the joined prefixes with the trailing delimiter.
func (n Node) Prefix() string {
	return string(convertKey(n.prefixes, ""))
}

func (n Node) Node(name string) Node {
	if name == "" {
		panic("Node name can't be empty")
	}

	prefixes := make([][]byte, 0, len(n.prefixes)+1)
	prefixes = append(prefixes, n.prefixes...)
	prefixes = append(prefixes, []byte(name))

	return Node{
		prefixes: prefixes,
		kvdb:     n.kvdb,
	}
}

func (n Node) SetBatch(f func(Batcher) error) error {
	batch := n.kvdb.db.NewWriteBatch()
	batch.SetMaxPendingTxns(32)

	if err := f(Batcher{n, batch}); err != nil {
		batch.Cancel()
		return err
	}

	return batch.Flush()
}

func (n Node) Set(k string, v interface{}) error {
	b, err := n.kvdb.Marshal(v)
	if err != nil {
		return errors.Wrap(err, "Failed to marshal")
	}

	key := convertKey(n.prefixes, k)

	return wrapErr("Failed to update db", n.kvdb.db.Update(
		func(tx *badger.Txn) error {
			return tx.Set(key, b)
		},
	))
}

func (n Node) SetWithTTL(k string, v interface{}, ttl time.Duration) error {
	b, err := n.kvdb.Marshal(v)
	if err != nil {
		return errors.Wrap(err, "Failed to marshal")
	}

	key := convertKey(n.prefixes, k)
	expiry := time.Now().Add(ttl)

	return wrapErr("Failed to update db", n.kvdb.db.Update(
		func(tx *badger.Txn) error {
			return tx.SetEntry(&badger.Entry{
				Key:       key,
				Value:     b,
				ExpiresAt: uint64(expiry.Unix()),
			})
		},
	))
}

func (n Node) Get(k string, v interface{}) error {
	key := convertKey(n.prefixes, k)

	return wrapErr("Failed to get from db", n.kvdb.db.View(
		func(tx *badger.Txn) error {
			i, err := tx.Get(key)
			if err != nil {
				if err != badger.ErrKeyNotFound {
					// Unknown error, log:
					log.Println("[db]: error:", err)
				}

				return err
			}

			return i.Value(func(b []byte) error {
				if err := n.kvdb.Unmarshal(b, v); err != nil {
					return errors.Wrap(err, "Failed to unmarshal")
				}
				return nil
			})
		},
	))
}

func (n Node) Delete(k string) error {
	key := convertKey(n.prefixes, k)

	return wrapErr("Failed to delete from db", n.kvdb.db.Update(
		func(tx *badger.Txn) error {
			return tx.Delete(key)
		},
	))
}

// Drop drops the entire node and all its values.
func (n Node) Drop() error {
	prefix := convertKey(n.prefixes, "")

	return wrapErr("Failed to delete from db", n.kvdb.db.Update(
		func(tx *badger.Txn) error {
			iter := tx.NewIterator(iterKeyOnlyOpts(prefix))
			defer iter.Close()

			var deleted bool

			for iter.Rewind(); iter.Valid(); iter.Next() {
				key := iter.Item().KeyCopy(nil)
				if err := tx.Delete(key); err != nil {
					return errors.Wrap(err, "Failed to delete key "+string(key))
				}

				deleted = true
			}

			if !deleted {
				return errors.New("nothing was deleted")
			}

			return nil
		},
	))
}

// All scans all values with the key prefix into the slice. This method uses
// reflection.
func (n Node) All(slicePtr interface{}, prefix string) error {
	vPtr := reflect.ValueOf(slicePtr)
	v := vPtr.Elem()

	if v.Kind() != reflect.Slice {
		return errors.New("not a slice")
	}

	// Grab the slice element's underlying type.
	var elemT = v.Type().Elem()
	var elemPtr = false
	if elemT.Kind() == reflect.Ptr {
		elemT = elemT.Elem()
		elemPtr = true
	}

	// this will have a trailing delimiter regardless
	longPrefix := convertKey(n.prefixes, prefix)

	fn := func(tx *badger.Txn) error {
		iter := tx.NewIterator(iterOpts(longPrefix))
		defer iter.Close()

		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()

			// Create a new element pointer
			var elem = reflect.New(elemT)

			// Start to unmarshal
			if err := item.Value(func(b []byte) error {
				return n.kvdb.Unmarshal(b, elem.Interface())
			}); err != nil {
				return errors.Wrap(err,
					"Failed to unmarshal into new underlying value")
			}

			// Check if dereference is needed before appending.
			if !elemPtr {
				// If the slice's underlying type is not a pointer,
				// dereference it.
				elem = elem.Elem()
			}

			// Append
			v.Set(reflect.Append(v, elem))
		}

		return nil
	}

	return wrapErr("Failed to iterate", n.kvdb.db.View(fn))
}

// EachBreak is an error that Each callbacks could return to stop the loop and
// return nil.
var EachBreak = errors.New("each break (not an error)")

// Each iterates over the bucket all possible keys with the prefix, or no
// prefix. It takes in a pointer.
//
// Caveats
//
// Since the pointer is reused, the user will need to manually copy it if they
// want to store the reference to that matched struct. Key includes the prefix.
//
// Example
//
// For iterating, as mentioned above, the user will need to manually copy the
// pointer by dereferencing and re-referencing it.
//
//    obj  :=   &Struct{}
//    objs := []*Struct{}
//
//    n.Each(obj, "", func(k string) error {
//        if obj.Thing == "what I want" {
//            cpy := *obj // copy
//            objs = append(objs, &cpy)
//        }
//
//        return nil
//    })
//
func (n Node) Each(v interface{}, prefix string, fn func(k string) error) error {
	// this will have a trailing delimiter regardless
	fullPrefix := convertKey(n.prefixes, "")

	return wrapErr("Failed to iterate", n.kvdb.db.View(
		func(tx *badger.Txn) error {
			iter := tx.NewIterator(iterOpts(appendString(fullPrefix, prefix)))
			defer iter.Close()

			for iter.Rewind(); iter.Valid(); iter.Next() {
				item := iter.Item()
				k := string(bytes.TrimPrefix(item.Key(), fullPrefix))

				if err := item.Value(func(b []byte) error {
					return n.kvdb.Unmarshal(b, v)
				}); err != nil {
					return errors.Wrap(err, "Failed to unmarshal "+k)
				}

				if err := fn(k); err != nil {
					if err == EachBreak {
						return nil
					}

					return err
				}
			}

			return nil
		},
	))
}

func (n Node) EachKey(prefix string, fn func(k string) error) error {
	fullPrefix := convertKey(n.prefixes, "")

	return wrapErr("Failed to iterate keys", n.kvdb.db.View(
		func(tx *badger.Txn) error {
			iter := tx.NewIterator(iterOpts(appendString(fullPrefix, prefix)))
			defer iter.Close()

			for iter.Rewind(); iter.Valid(); iter.Next() {
				item := iter.Item()
				k := string(bytes.TrimPrefix(item.Key(), fullPrefix))

				if err := fn(k); err != nil {
					if err == EachBreak {
						return nil
					}

					return err
				}
			}

			return nil
		},
	))
}
