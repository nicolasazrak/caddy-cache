package storage

import (
	"time"
	"sync"
	"hash/crc32"
)

const bucketsSize = 256

type MemoryStorage struct {
	URL string
	contents [bucketsSize]map[string]CacheEntry
	mutex [bucketsSize]*sync.RWMutex
}

type CacheEntry struct {
	value *CachedResponse
	expiration time.Time
}

func (s *MemoryStorage) getBucketIndexForKey(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key)) & bucketsSize
}

func (s *MemoryStorage) Get(key string) (*CachedResponse, error) {
	i := s.getBucketIndexForKey(key)

	s.mutex[i].RLock()
	defer s.mutex[i].RUnlock()

	entry, ok := s.contents[i][key]
	if !ok {
		return nil, nil
	}
	return entry.value, nil
}

func (s *MemoryStorage) Set(key string, cached * CachedResponse, expiration time.Time) error {
	i := s.getBucketIndexForKey(key)

	s.mutex[i].Lock()
	defer s.mutex[i].Unlock()

	s.contents[i][key] = CacheEntry{
		value: cached,
		expiration: expiration,
	}
	return nil
}

func (s *MemoryStorage) Setup() error {
	for i := 0; i < int(bucketsSize); i++ {
		s.mutex[i] = new(sync.RWMutex)
		s.contents[i] = make(map[string]CacheEntry)
	}
	go doEvery(time.Duration(1) * time.Second, s.expire)
	return nil
}

func (s *MemoryStorage) expire() {
	for i := 0; i < int(bucketsSize); i++ {
		s.mutex[i].Lock()
		for k := range s.contents[i] {
			if s.contents[i][k].expiration.Sub(time.Now().UTC()) < 0 {
				delete(s.contents[i], k)
			}
		}
		s.mutex[i].Unlock()
	}
}

func doEvery(d time.Duration, f func()) {
	for range time.Tick(d) {
		f()
	}
}