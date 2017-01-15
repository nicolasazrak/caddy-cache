package cache

import (
	"hash/crc32"
	"math"
	"sync"
	"time"
)

const bucketsSize = 256

type Cache struct {
	// This are the Read/Write locks shared by key
	// So the prevents to lock all keys when only one is being written
	entriesLock [bucketsSize]*sync.RWMutex
	entries     [bucketsSize]map[string]*CacheEntry
	storage     Storage
}

type CacheEntry struct {
	// This lock is used only to prevent concurrent access to the same upstream
	// It is not meant to protect access to values, the `mutex` of storage is used for that.
	valuesLock *sync.RWMutex
	values     []*Value
}

type Value struct {
	expiration time.Time

	// This lock prevents deleting the content on disk
	// While there is a request reading from it
	refLock *sync.RWMutex

	// ref is a previous request/response stored
	// This can easily be a Clear() interface but
	// Using HttpCacheEntry prevents having to cast
	// everywhere. This is because the lock of the map
	// Is used to prevent deleting it from the map
	// But the lock is released after the entry is read from the map
	// It does not mean that some go routine is not using it
	ref *HttpCacheEntry
}

func NewCache(storage Storage) *Cache {
	return &Cache{storage: storage}
}

func (s *Cache) Setup() error {
	for i := 0; i < int(bucketsSize); i++ {
		s.entriesLock[i] = new(sync.RWMutex)
		s.entries[i] = make(map[string]*CacheEntry)
	}

	return s.storage.Setup()
}

func (s *Cache) Clear() error {
	// TODO
	return nil
	// return s.storage.Clean()
}

func (s *Cache) getEntry(key string) *CacheEntry {
	i := s.getBucketIndexForKey(key)

	s.entriesLock[i].Lock()
	entry, ok := s.entries[i][key]
	if !ok {
		s.entries[i][key] = &CacheEntry{
			values:     []*Value{},
			valuesLock: new(sync.RWMutex),
		}
		entry = s.entries[i][key]
	}
	s.entriesLock[i].Unlock()

	return entry
}

/**
 * Given a key and a matcher it fetches the searched value.
 * If it exists it calls the handler with that value.
 * If it doesn't it calls the value with nil and the return value
 * of the handler will be pushed.
 */
func (s *Cache) GetOrSet(key string, condition func(*HttpCacheEntry) bool, handler func(*HttpCacheEntry) (*HttpCacheEntry, error)) error {
	entry := s.getEntry(key)

	// While iterating throw the values is important that nobody else writes on it
	// Until the resource is found in the entries' list or is fetched
	entry.valuesLock.Lock()

	for _, value := range entry.values {
		if condition(value.ref) {
			// The searched resource if found, the list can be unlocked
			entry.valuesLock.Unlock()

			// Read lock the content so it is not expired while using it in the handler
			value.refLock.RLock()
			//noinspection GoDeferInLoop
			defer value.refLock.RUnlock()

			// Call the handler
			newValue, err := handler(value.ref)

			// The case when newValue is not nil is when a previous time called
			// was not cacheable but now it is. Should rarely happen
			if err == nil && newValue != nil {
				entry.valuesLock.Lock()
				s.unsafePush(key, entry, newValue)
				entry.valuesLock.Unlock()
			}

			return err
		}
	}

	// If the entry is not on the list wait until it is fetched from upstream
	defer entry.valuesLock.Unlock()

	newValue, err := handler(nil)
	if err != nil || newValue == nil {
		return err
	}

	s.unsafePush(key, entry, newValue)
	return nil
}

func (s *Cache) unsafePush(key string, entry *CacheEntry, newValue *HttpCacheEntry) {
	value := &Value{
		ref:        newValue,
		refLock:    new(sync.RWMutex),
		expiration: newValue.Expiration,
	}

	// This pushes the new entry on top of the slice
	// This is useful to use the most recent values first
	entry.values = append([]*Value{value}, entry.values...)

	// Launch a new go routine that will expire the content
	go s.expire(key, newValue.Expiration)
}

func (s *Cache) getBucketIndexForKey(key string) uint32 {
	return uint32(math.Mod(float64(crc32.ChecksumIEEE([]byte(key))), float64(bucketsSize)))
}

func (s *Cache) NewContent(key string) (StorageContent, error) {
	return s.storage.NewContent(key)
}

func (s *Cache) expire(key string, expiration time.Time) {
	// Sleep until is time to cleanup entry
	time.Sleep(expiration.Sub(time.Now().UTC()))

	// Get bucket and lock
	bucket := s.getBucketIndexForKey(key)
	s.entriesLock[bucket].Lock()
	entry, ok := s.entries[bucket][key]
	s.entriesLock[bucket].Unlock()

	if !ok {
		// Possible bug, how it ended here?
		return
	}

	entry.valuesLock.Lock()
	defer entry.valuesLock.Unlock()

	// This should prevent creating a new array
	// Uses the same slice
	notExpiredValues := entry.values[:0]
	for _, value := range entry.values {
		// Check which entry for the key is expired
		if value.expiration.After(time.Now().UTC()) {
			notExpiredValues = append(notExpiredValues, value)
		} else {
			// Clear the content in other go routine
			// If it is being red it can block others
			go func(value *Value) {
				// Get lock to prevent any other go routine read from it
				value.refLock.Lock()
				// Delete it if it is on disk
				value.ref.Clear()
			}(value) // Copying the pointer to the go routine is required to avoid reading an invalid value
		}
	}
	entry.values = notExpiredValues
}
