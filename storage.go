package cache

import (
	"hash/crc32"
	"math"
	"sync"
	"time"
)

const bucketsSize = 256

type Value interface{}

type MemoryStorage struct {
	contents [bucketsSize]map[string][]*CacheEntry

	// This are the Read/Write locks shared by key
	// So the prevents to lock all keys when only one is being written
	mutex [bucketsSize]*sync.RWMutex
}

type CacheEntry struct {
	value      Value
	expiration time.Time
}

func (s *MemoryStorage) getBucketIndexForKey(key string) uint32 {
	return uint32(math.Mod(float64(crc32.ChecksumIEEE([]byte(key))), float64(bucketsSize)))
}

func (s *MemoryStorage) Get(key string, matcher func(Value) bool) (Value, error) {
	i := s.getBucketIndexForKey(key)
	s.mutex[i].RLock()
	defer s.mutex[i].RUnlock()

	entries, ok := s.contents[i][key]
	if !ok {
		return nil, nil
	}

	for _, entry := range entries {
		if matcher(entry.value) {
			return entry.value, nil
		}
	}

	return nil, nil
}

func (s *MemoryStorage) Push(key string, cached Value, expiration time.Time) error {
	i := s.getBucketIndexForKey(key)

	s.mutex[i].Lock()
	defer s.mutex[i].Unlock()

	entries, ok := s.contents[i][key]
	newEntry := &CacheEntry{
		value:      cached,
		expiration: expiration,
	}

	if ok {
		s.contents[i][key] = append(entries, newEntry)
	} else {
		s.contents[i][key] = []*CacheEntry{newEntry}
	}

	// Launch a new go routine that will expire the content
	go s.expire(key, expiration)

	return nil
}

func (s *MemoryStorage) Setup() error {
	for i := 0; i < int(bucketsSize); i++ {
		s.mutex[i] = new(sync.RWMutex)
		s.contents[i] = make(map[string][]*CacheEntry)
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

	// This should prevent creating a new array
	// Uses the same slice
	notExpiredContent := s.contents[bucket][key][:0]
	for _, entry := range s.contents[bucket][key] {
		// Check which entry for the key is expired
		if entry.expiration.After(time.Now().UTC()) {
			notExpiredContent = append(notExpiredContent, entry)
		}
	}
	s.contents[bucket][key] = notExpiredContent
}
