// API of key value store to be used in server_impl

package kvstore

// KVStore -- Interface for Key/Value stores
type KVStore interface {
	// Put inserts a new key value pair or updates the value for a
	// given key in the store
	Put(key string, value []byte)

	// Get fetches the value associated with the key
	Get(key string) []([]byte)

	// Clear removes all values associated with the key
	Clear(key string)
}
