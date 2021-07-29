package db

import (
	"runtime"

	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
	"github.com/pkg/errors"
)

const delimiter = "\x00"

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
	namesBytes := make([][]byte, 0, len(p)+len(tails))
	namesBytes = append(namesBytes, p...)
	for i := range tails {
		namesBytes = append(namesBytes, []byte(tails[i]))
	}
	return namesBytes
}

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
	optimumWorkers := halfMin(runtime.GOMAXPROCS(-1), 1)

	opt := badger.LSMOnlyOptions(path)
	opt = opt.WithNumGoroutines(optimumWorkers)
	opt = opt.WithNumCompactors(optimumWorkers)
	opt = opt.WithLoggingLevel(badger.WARNING)
	opt = opt.WithCompression(options.ZSTD)
	opt = opt.WithZSTDCompressionLevel(2)
	opt = opt.WithBlockCacheSize(1 << 24)   // 16MB
	opt = opt.WithValueLogFileSize(1 << 29) // 500MB
	opt = opt.WithCompactL0OnClose(true)
	opt = opt.WithMetricsEnabled(false)

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
		Marshaler: CBORMarshaler,
		db:        db,
	}
}

func (kv KV) NodeFromPath(path NodePath) Node {
	return Node{
		prefixes: path,
		kv:       &kv,
	}
}

func (kv KV) Node(names ...string) Node {
	if len(names) == 0 {
		panic("Node name can't be empty")
	}

	return kv.NodeFromPath(NewNodePath(names...))
}

func (kv KV) Close() error {
	return kv.db.Close()
}
