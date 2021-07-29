package db

import (
	"bytes"
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

func iterValidKey(iter *badger.Iterator, prefix []byte) bool {
	if !iter.Valid() {
		return false
	}

	k := iter.Item().Key()
	k = bytes.TrimPrefix(k, prefix)
	return !bytes.Contains(k, []byte(delimiter))
}

func iterSplitKey(iter *badger.Iterator, prefix []byte) ([]byte, bool) {
	k := iter.Item().Key()
	k = bytes.TrimPrefix(k, prefix)

	i := bytes.Index(k, []byte(delimiter))
	if i == -1 {
		return k, true
	}

	return nil, false
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
		// log.Println("unexpected db error:", err)
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
	kv       *KV
	txn      *badger.Txn
	prefixes [][]byte
}

// TxUpdate creates a new Node with an active transaction and calls f. If this
// method is called in a Node that already has a transaction, then that
// transaction is reused.
func (n Node) TxUpdate(f func(n Node) error) error {
	if n.txn != nil {
		return f(n)
	}

	n.txn = n.kv.db.NewTransaction(true)
	defer n.txn.Discard()

	if err := f(n); err != nil {
		return err
	}

	return n.txn.Commit()
}

// TxUpdate creates a new Node with an active read-only transaction and calls f.
// If this method is called in a Node that already has a transaction, then that
// transaction is reused.
func (n Node) TxView(f func(n Node) error) error {
	if n.txn != nil {
		return f(n)
	}

	n.txn = n.kv.db.NewTransaction(false)
	defer n.txn.Discard()

	if err := f(n); err != nil {
		return err
	}

	return n.txn.Commit()
}

// Prefix returns the joined prefixes with the trailing delimiter.
func (n Node) Prefix() string {
	return string(convertKey(n.prefixes, ""))
}

// FromPath creates a new node with the given full path. The path will
// completely override the old path.
func (n Node) FromPath(path NodePath) Node {
	n.prefixes = path
	return n
}

// Node creates a child node with the given names appended to its path. If the
// node has an ongoing transaction, then it is inherited over.
func (n Node) Node(names ...string) Node {
	if len(names) == 0 {
		panic("Node name can't be empty")
	}

	namesBytes := make([][]byte, len(names))
	for i := range names {
		namesBytes[i] = []byte(names[i])
	}

	prefixes := make([][]byte, 0, len(n.prefixes)+1)
	prefixes = append(prefixes, n.prefixes...)
	prefixes = append(prefixes, namesBytes...)

	n.prefixes = prefixes
	return n
}

func (n Node) Set(k string, v interface{}) error {
	b, err := n.kv.Marshal(v)
	if err != nil {
		return errors.Wrap(err, "Failed to marshal")
	}

	key := convertKey(n.prefixes, k)

	return wrapErr("failed to update db", n.TxUpdate(
		func(n Node) error {
			return n.txn.Set(key, b)
		},
	))
}

func (n Node) SetWithTTL(k string, v interface{}, ttl time.Duration) error {
	b, err := n.kv.Marshal(v)
	if err != nil {
		return errors.Wrap(err, "Failed to marshal")
	}

	key := convertKey(n.prefixes, k)
	expiry := time.Now().Add(ttl)

	return wrapErr("failed to update db", n.TxUpdate(
		func(n Node) error {
			return n.txn.SetEntry(&badger.Entry{
				Key:       key,
				Value:     b,
				ExpiresAt: uint64(expiry.Unix()),
			})
		},
	))
}

// Exists returns true if the given key exists.
func (n Node) Exists(k string) bool {
	return n.Get(k, nil) == nil
}

func (n Node) Get(k string, v interface{}) error {
	key := convertKey(n.prefixes, k)

	return wrapErr("failed to get from db", n.TxView(
		func(n Node) error {
			i, err := n.txn.Get(key)
			if err != nil {
				return err
			}

			if v == nil {
				return nil
			}

			return i.Value(func(b []byte) error {
				if err := n.kv.Unmarshal(b, v); err != nil {
					return errors.Wrap(err, "failed to unmarshal")
				}
				return nil
			})
		},
	))
}

func (n Node) Delete(k string) error {
	key := convertKey(n.prefixes, k)

	return wrapErr("failed to delete from db", n.TxUpdate(
		func(n Node) error {
			return n.txn.Delete(key)
		},
	))
}

// Drop drops the entire node and all its values.
func (n Node) Drop() error {
	prefix := convertKey(n.prefixes, "")

	return wrapErr("failed to delete from db", n.TxUpdate(
		func(n Node) error {
			iter := n.txn.NewIterator(iterKeyOnlyOpts(prefix))
			defer iter.Close()

			for iter.Rewind(); iter.Valid(); iter.Next() {
				key := iter.Item().KeyCopy(nil)
				if err := n.txn.Delete(key); err != nil {
					return errors.Wrap(err, "failed to delete key "+string(key))
				}
			}

			return nil
		},
	))
}

// DropExceptLast drops the entire node except for the last few values. This
// method heavily relies on keyed values being sorted properly, and that the
// stored values are NOT nested.
func (n Node) DropExceptLast(last int) error {
	prefix := convertKey(n.prefixes, "")

	var total int

	return wrapErr("failed to delete from db", n.TxUpdate(
		func(n Node) error {
			iter := n.txn.NewIterator(iterKeyOnlyOpts(prefix))
			defer iter.Close()

			for iter.Rewind(); iter.Valid(); iter.Next() {
				total++
			}

			until := total - last
			if until < 1 {
				return nil
			}

			var deleted int

			for iter.Rewind(); iter.Valid() && until > deleted; iter.Next() {
				key := iter.Item().KeyCopy(nil)
				if err := n.txn.Delete(key); err != nil {
					return errors.Wrapf(err, "failed to delete key %q", key)
				}
				deleted++
			}

			return nil
		},
	))
}

// All scans all values with the key prefix into the slice. This method uses
// reflection. The given slice will have its length reset to 0.
func (n Node) All(slicePtr interface{}, prefix string) error {
	vPtr := reflect.ValueOf(slicePtr)
	if vPtr.Kind() != reflect.Ptr {
		return errors.New("given slice ptr is not a ptr")
	}

	v := vPtr.Elem()
	if v.Kind() != reflect.Slice {
		return errors.New("not a slice")
	}

	// this will have a trailing delimiter regardless
	longPrefix := convertKey(n.prefixes, prefix)

	fn := func(n Node) error {
		iter := n.txn.NewIterator(iterOpts(longPrefix))
		defer iter.Close()

		var length int
		for iter.Rewind(); iterValidKey(iter, longPrefix); iter.Next() {
			length++
		}

		if length == 0 {
			v.SetLen(0)
			return nil
		}

		// Reallocate anyway, because we want fresh zero values.
		vType := v.Type()
		v.Set(reflect.MakeSlice(vType, length, length))

		var ix int
		for iter.Rewind(); iterValidKey(iter, longPrefix); iter.Next() {
			item := iter.Item()
			// Directly use a pointer to the backing array to unmarshal into.
			dst := v.Index(ix).Addr()
			ix++

			// Start to unmarshal
			if err := item.Value(func(b []byte) error {
				return n.kv.Unmarshal(b, dst.Interface())
			}); err != nil {
				return errors.Wrap(err, "failed to unmarshal into new underlying value")
			}
		}

		return nil
	}

	return wrapErr("failed to iterate", n.TxView(fn))
}

// Length queries the number of keys within the node, similarly to running
// AllKeys and taking the length of what was returned.
func (n Node) Length(prefix string) (int, error) {
	// this will have a trailing delimiter regardless
	longPrefix := convertKey(n.prefixes, prefix)
	var length int

	return length, wrapErr("failed to iterate keys", n.TxView(
		func(n Node) error {
			iter := n.txn.NewIterator(iterKeyOnlyOpts(longPrefix))
			defer iter.Close()

			for iter.Rewind(); iterValidKey(iter, longPrefix); iter.Next() {
				length++
			}

			return nil
		},
	))
}

var stringType = reflect.TypeOf("")

// AllKeys is similar to All, except only the keys are fetched.
func (n Node) AllKeys(slicePtr interface{}, prefix string) error {
	vPtr := reflect.ValueOf(slicePtr)
	if vPtr.Kind() != reflect.Ptr {
		return errors.New("given slice ptr is not a ptr")
	}

	v := vPtr.Elem()
	if v.Kind() != reflect.Slice {
		return errors.New("not a slice")
	}

	elemT := v.Type().Elem()
	needsConvert := stringType == elemT

	// this will have a trailing delimiter regardless
	longPrefix := convertKey(n.prefixes, prefix)

	return wrapErr("failed to iterate keys", n.TxView(
		func(n Node) error {
			iter := n.txn.NewIterator(iterKeyOnlyOpts(longPrefix))
			defer iter.Close()

			var length int
			for iter.Rewind(); iterValidKey(iter, longPrefix); iter.Next() {
				length++
			}

			if length == 0 {
				v.SetLen(0)
				return nil
			}

			if v.Cap() < length {
				vType := v.Type()
				v.Set(reflect.MakeSlice(vType, length, length))
			} else {
				v.SetLen(length)
			}

			var ix int

			for iter.Rewind(); iter.Valid(); iter.Next() {
				ik, ok := iterSplitKey(iter, longPrefix)
				if !ok {
					continue
				}

				vk := reflect.ValueOf(string(ik))
				if needsConvert {
					vk = vk.Convert(elemT)
				}

				v.Index(ix).Set(vk)
				ix++
			}

			return nil
		},
	))
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
func (n Node) Each(v interface{}, prefix string, fn func(k string, l int) error) error {
	// this will have a trailing delimiter regardless
	fullPrefix := convertKey(n.prefixes, prefix)

	return wrapErr("failed to iterate", n.TxView(
		func(n Node) error {
			iter := n.txn.NewIterator(iterOpts(fullPrefix))
			defer iter.Close()

			var length int
			for iter.Rewind(); iterValidKey(iter, fullPrefix); iter.Next() {
				length++
			}

			for iter.Rewind(); iter.Valid(); iter.Next() {
				ik, ok := iterSplitKey(iter, fullPrefix)
				if !ok {
					continue
				}

				item := iter.Item()

				if err := item.Value(func(b []byte) error {
					return n.kv.Unmarshal(b, v)
				}); err != nil {
					return errors.Wrapf(err, "failed to unmarshal %q", string(ik))
				}

				if err := fn(string(ik), length); err != nil {
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

// EachKey iterates over keys.
func (n Node) EachKey(prefix string, fn func(k string, l int) error) error {
	fullPrefix := convertKey(n.prefixes, prefix)

	return wrapErr("failed to iterate keys", n.TxView(
		func(n Node) error {
			iter := n.txn.NewIterator(iterOpts(fullPrefix))
			defer iter.Close()

			var length int
			for iter.Rewind(); iterValidKey(iter, fullPrefix); iter.Next() {
				length++
			}

			for iter.Rewind(); iter.Valid(); iter.Next() {
				ik, ok := iterSplitKey(iter, fullPrefix)
				if !ok {
					continue
				}

				if err := fn(string(ik), length); err != nil {
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
