package storage

import (
	"time"
	"runtime/debug"
	"github.com/coocood/freecache"
)

type MemoryStorage struct {
	URL string
	client *freecache.Cache
}


func (s *MemoryStorage) Get(key string) (*CachedResponse, error) {
	val, err := s.client.Get([]byte(key))
	if err != nil && err != freecache.ErrNotFound {
		return nil, err
	}
	if err == freecache.ErrNotFound {
		return nil, nil
	}
	return unmarshall(val)
}

func (s *MemoryStorage) Set(key string, cached * CachedResponse, expiration time.Duration) error {
	val, err := marshal(cached)
	if err != nil {
		return err
	}
	return s.client.Set([]byte(key), val, int(expiration.Seconds()))
}

func (s *MemoryStorage) Setup() error {
	size := 1024 * 1024 * 1024
	s.client = freecache.NewCache(size)
	debug.SetGCPercent(10)
	return nil
}
