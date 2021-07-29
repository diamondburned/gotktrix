package db

// // Batcher wraps around a single node to batch write to it. The node can be
// // swapped out using WithNode.
// type Batcher struct {
// 	n Node
// 	b *badger.WriteBatch
// }

// // WithNode creates a copy of Batcher with the given node..
// func (batcher Batcher) WithNode(n Node) Batcher {
// 	batcher.n = n
// 	return batcher
// }

// func (batcher Batcher) Set(k string, v interface{}) error {
// 	b, err := batcher.n.kvdb.Marshal(v)
// 	if err != nil {
// 		return errors.Wrap(err, "Failed to marshal")
// 	}

// 	key := convertKey(batcher.n.prefixes, k)

// 	return batcher.b.Set(key, b)
// }

// func (batcher Batcher) SetWithTTL(k string, v interface{}, ttl time.Duration) error {
// 	b, err := batcher.n.kvdb.Marshal(v)
// 	if err != nil {
// 		return errors.Wrap(err, "Failed to marshal")
// 	}

// 	key := convertKey(batcher.n.prefixes, k)
// 	expiry := time.Now().Add(ttl)

// 	return batcher.b.SetEntry(&badger.Entry{
// 		Key:       key,
// 		Value:     b,
// 		ExpiresAt: uint64(expiry.Unix()),
// 	})
// }

// func (batcher Batcher) Delete(k string) error {
// 	key := convertKey(batcher.n.prefixes, k)
// 	return batcher.b.Delete(key)
// }
