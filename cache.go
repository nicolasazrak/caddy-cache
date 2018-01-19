package cache

import (
	"hash/crc32"
	"math"
	"net/http"
	"sync"
	"time"
)

const cacheBucketsSize = 256

type HTTPCache struct {
	entries     [cacheBucketsSize]map[string][]*HTTPCacheEntry
	entriesLock [cacheBucketsSize]*sync.RWMutex
}

func NewHTTPCache() *HTTPCache {
	entriesLocks := [cacheBucketsSize]*sync.RWMutex{}
	entries := [cacheBucketsSize]map[string][]*HTTPCacheEntry{}

	for i := 0; i < int(cacheBucketsSize); i++ {
		entriesLocks[i] = new(sync.RWMutex)
		entries[i] = make(map[string][]*HTTPCacheEntry)
	}

	return &HTTPCache{
		entries:     entries,
		entriesLock: entriesLocks,
	}
}

func (cache *HTTPCache) Get(request *http.Request) (*HTTPCacheEntry, bool) {
	entry, freshness := cache.GetFreshness(request, getKey(request))
	return entry, freshness == 0
}

func (cache *HTTPCache) GetFreshness(request *http.Request, key string) (*HTTPCacheEntry, int) {
	b := cache.getBucketIndexForKey(key)
	cache.entriesLock[b].RLock()
	defer cache.entriesLock[b].RUnlock()

	previousEntries, exists := cache.entries[b][key]

	if !exists {
		return nil, 1
	}

	for _, entry := range previousEntries {
		if entry.Fresh() && matchesVary(request, entry) {
			return entry, 0
		}
	}

	return nil, 2
}

func (cache *HTTPCache) Put(request *http.Request, entry *HTTPCacheEntry) {
	key := entry.Key()
	bucket := cache.getBucketIndexForKey(key)

	cache.entriesLock[bucket].Lock()
	defer cache.entriesLock[bucket].Unlock()

	cache.scheduleCleanEntry(entry)

	for i, previousEntry := range cache.entries[bucket][key] {
		if matchesVary(entry.Request, previousEntry) {
			go previousEntry.Clean()
			cache.entries[bucket][key][i] = entry
			return
		}
	}

	cache.entries[bucket][key] = append(cache.entries[bucket][key], entry)
}

func (cache *HTTPCache) scheduleCleanEntry(entry *HTTPCacheEntry) {
	go func(entry *HTTPCacheEntry) {
		time.Sleep(entry.expiration.Sub(time.Now().UTC()))
		cache.cleanEntry(entry)
	}(entry)
}

func (cache *HTTPCache) cleanEntry(entry *HTTPCacheEntry) {
	key := entry.Key()
	bucket := cache.getBucketIndexForKey(key)

	cache.entriesLock[bucket].Lock()
	defer cache.entriesLock[bucket].Unlock()

	for i, otherEntry := range cache.entries[bucket][key] {
		if entry == otherEntry {
			cache.entries[bucket][key] = append(cache.entries[bucket][key][:i], cache.entries[bucket][key][i+1:]...)
			entry.Clean()
			return
		}
	}
}

func (cache *HTTPCache) getBucketIndexForKey(key string) uint32 {
	return uint32(math.Mod(float64(crc32.ChecksumIEEE([]byte(key))), float64(cacheBucketsSize)))
}
