package storage

import (
	"hash/crc32"
	"math"
	"sync"
	"time"
)

const bucketsSize = 256

type MemoryStorage struct {
	contents [bucketsSize]map[string][]*CacheEntry
	mutex    [bucketsSize]*sync.RWMutex
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

	return nil
}

func (s *MemoryStorage) Setup() error {
	for i := 0; i < int(bucketsSize); i++ {
		s.mutex[i] = new(sync.RWMutex)
		s.contents[i] = make(map[string][]*CacheEntry)
	}
	go doEvery(time.Duration(1)*time.Second, s.expire)
	return nil
}

// TODO test this
func (s *MemoryStorage) expire() {
	for bucket := 0; bucket < int(bucketsSize); bucket++ {
		s.mutex[bucket].Lock()
		for key := range s.contents[bucket] {
			notExpiredContent := s.contents[bucket][key][:0]
			for _, entry := range s.contents[bucket][key] {
				if entry.expiration.After(time.Now().UTC()) {
					notExpiredContent = append(notExpiredContent, entry)
				}
			}
			s.contents[bucket][key] = notExpiredContent
		}
		s.mutex[bucket].Unlock()
	}
}

func doEvery(d time.Duration, f func()) {
	for range time.Tick(d) {
		f()
	}
}
