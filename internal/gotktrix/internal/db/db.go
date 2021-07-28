package db

import (
	"runtime"

	"github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"
)

const delimiter = "\x00"

type KV struct {
	Marshaler
	db *badger.DB
}

func halfMin(v, min int) int {
	v /= 2
	if v > min {
		return v
	}
	return min
}

func NewKVFile(path string) (*KV, error) {
	opt := badger.DefaultOptions(path)
	opt = opt.WithNumGoroutines(halfMin(runtime.GOMAXPROCS(-1), 1))
	opt = opt.WithLoggingLevel(badger.WARNING)
	opt = opt.WithZSTDCompressionLevel(2)
	return NewKV(opt)
}

func NewKV(opts badger.Options) (*KV, error) {
	b, err := badger.Open(opts)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open a Badger DB")
	}

	return KVWithDB(b), nil
}

func KVWithDB(db *badger.DB) *KV {
	return &KV{
		Marshaler: JSONMarshaler,
		db:        db,
	}
}

func (kv KV) SetBatch(f func(Batcher) error) error {
	batch := kv.db.NewWriteBatch()
	batch.SetMaxPendingTxns(32)

	if err := f(Batcher{Node{kvdb: kv}, batch}); err != nil {
		batch.Cancel()
		return err
	}

	return batch.Flush()
}

func (kv KV) Node(name string) Node {
	if name == "" {
		panic("Node name can't be empty")
	}

	return Node{
		prefixes: [][]byte{[]byte(name)},
		kvdb:     kv,
	}
}

func (kv KV) Close() error {
	return kv.db.Close()
}
