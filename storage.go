package cache

import (
	"hash/crc32"
	"math"
	"sync"
	"time"
)

const bucketsSize = 256

type MemoryStorage struct {
	contents [bucketsSize]map[string]*CacheEntry

	// This are the Read/Write locks shared by key
	// So the prevents to lock all keys when only one is being written
	mutex [bucketsSize]*sync.RWMutex
}

type CacheEntry struct {
	// This lock is used only to prevent concurrent access to the same upstream
	// It is not meant to protect access to values, the `mutex` of storage is used for that.
	lock *sync.Mutex

	values []*HttpCacheEntry // Values are each previous request stored
}

func (s *MemoryStorage) getEntry(key string) *CacheEntry {
	i := s.getBucketIndexForKey(key)

	s.mutex[i].Lock()
	entry, ok := s.contents[i][key]
	if !ok {
		s.contents[i][key] = &CacheEntry{
			values: []*HttpCacheEntry{},
			lock:   new(sync.Mutex),
		}
		entry = s.contents[i][key]
	}
	s.mutex[i].Unlock()

	return entry
}

func (s *MemoryStorage) GetOrLock(key string, condition func(*HttpCacheEntry) bool, handler func(*HttpCacheEntry) error) error {
	entry := s.getEntry(key)
	entry.lock.Lock()

	for _, value := range entry.values {
		if condition(value) {
			entry.lock.Unlock()
			return handler(value)
		}
	}

	defer entry.lock.Unlock()
	return handler(nil)
}

func (s *MemoryStorage) getBucketIndexForKey(key string) uint32 {
	return uint32(math.Mod(float64(crc32.ChecksumIEEE([]byte(key))), float64(bucketsSize)))
}

func (s *MemoryStorage) Push(key string, newEntry *HttpCacheEntry) error {
	i := s.getBucketIndexForKey(key)
	s.mutex[i].Lock()
	defer s.mutex[i].Unlock()

	entry, ok := s.contents[i][key]
	if !ok {
		s.contents[i][key] = &CacheEntry{
			lock: new(sync.Mutex),
		}
		entry = s.contents[i][key]
	}

	if ok {
		// This pushes the new entry on top of the slice
		// This is useful to use the most recent values first
		entry.values = append([]*HttpCacheEntry{newEntry}, entry.values...)
	} else {
		entry.values = []*HttpCacheEntry{newEntry}
	}

	// Launch a new go routine that will expire the content
	go s.expire(key, newEntry.Expiration)

	return nil
}

func (s *MemoryStorage) Setup() error {
	for i := 0; i < int(bucketsSize); i++ {
		s.mutex[i] = new(sync.RWMutex)
		s.contents[i] = make(map[string]*CacheEntry)
	}
	return nil
}

func (s *MemoryStorage) expire(key string, expiration time.Time) {
	// Sleep until is time to cleanup entry
	time.Sleep(expiration.Sub(time.Now().UTC()))

	// Get bucket and lock
	bucket := s.getBucketIndexForKey(key)
	s.mutex[bucket].Lock()
	defer s.mutex[bucket].Unlock()

	entry := s.contents[bucket][key]

	entry.lock.Lock()
	defer entry.lock.Unlock()

	// This should prevent creating a new array
	// Uses the same slice
	notExpiredContent := entry.values[:0]
	for _, entry := range entry.values {
		// Check which entry for the key is expired
		if entry.Expiration.After(time.Now().UTC()) {
			notExpiredContent = append(notExpiredContent, entry)
		}
	}
	entry.values = notExpiredContent
}
