package cache

import (
	"hash/crc32"
	"math"
	"sync"
)

const urlLockBucketsSize = 256

type URLLock struct {
	globalLocks [urlLockBucketsSize]*sync.Mutex
	keys        [urlLockBucketsSize]map[string]*sync.Mutex
}

func NewURLLock() *URLLock {
	globalLocks := [urlLockBucketsSize]*sync.Mutex{}
	keys := [urlLockBucketsSize]map[string]*sync.Mutex{}

	for i := 0; i < int(urlLockBucketsSize); i++ {
		globalLocks[i] = new(sync.Mutex)
		keys[i] = make(map[string]*sync.Mutex)
	}

	return &URLLock{
		globalLocks: globalLocks,
		keys:        keys,
	}
}

// Adquire a lock for given key
func (allLocks *URLLock) Adquire(key string) *sync.Mutex {
	bucketIndex := allLocks.getBucketIndexForKey(key)
	allLocks.globalLocks[bucketIndex].Lock()
	defer allLocks.globalLocks[bucketIndex].Unlock()

	lock, exists := allLocks.keys[bucketIndex][key]
	if !exists {
		lock = new(sync.Mutex)
		allLocks.keys[bucketIndex][key] = lock
	}
	lock.Lock()
	return lock
}

func (allLocks *URLLock) getBucketIndexForKey(key string) uint32 {
	return uint32(math.Mod(float64(crc32.ChecksumIEEE([]byte(key))), float64(urlLockBucketsSize)))
}
