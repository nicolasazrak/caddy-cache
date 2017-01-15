package cache

import (
	"bytes"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"math/rand"
	"os"
	"path"
	"sync"
	"syscall"
	"time"
)

const bucketsSize = 256

type Cache struct {
	contents [bucketsSize]map[string]*CacheEntry

	// This are the Read/Write locks shared by key
	// So the prevents to lock all keys when only one is being written
	mutex [bucketsSize]*sync.RWMutex

	storage Storage
}

type CacheEntry struct {
	// This lock is used only to prevent concurrent access to the same upstream
	// It is not meant to protect access to values, the `mutex` of storage is used for that.
	lock *sync.Mutex

	values []*HttpCacheEntry // Values are each previous request stored
}

func NewCache(storage Storage) *Cache {
	return &Cache{storage: storage}
}

func (s *Cache) Setup() error {
	for i := 0; i < int(bucketsSize); i++ {
		s.mutex[i] = new(sync.RWMutex)
		s.contents[i] = make(map[string]*CacheEntry)
	}

	return s.storage.Setup()
}

func (s *Cache) Clean() error {
	return s.storage.Clean()
}

func (s *Cache) getEntry(key string) *CacheEntry {
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

func (s *Cache) GetOrLock(key string, condition func(*HttpCacheEntry) bool, handler func(*HttpCacheEntry) error) error {
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

func (s *Cache) getBucketIndexForKey(key string) uint32 {
	return uint32(math.Mod(float64(crc32.ChecksumIEEE([]byte(key))), float64(bucketsSize)))
}

func (s *Cache) Push(key string, newEntry *HttpCacheEntry) {
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
}

func (s *Cache) NewEntry(key string) (io.ReadWriter, error) {
	return s.storage.NewEntry(key)
}

func (s *Cache) CloseEntry(entry io.ReadWriter) ([]byte, error) {
	return s.storage.CloseEntry(entry)
}

func (s *Cache) expire(key string, expiration time.Time) {
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

/* Storage */
type Storage interface {
	Setup() error
	Clean() error
	NewEntry(string) (io.ReadWriter, error)
	CloseEntry(io.ReadWriter) ([]byte, error)
}

/* Memory Storage */

type MemoryStorage struct{}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

func (s *MemoryStorage) Setup() error {
	return nil
}

func (s *MemoryStorage) Clean() error {
	return nil
}

func (s *MemoryStorage) NewEntry(key string) (io.ReadWriter, error) {
	return new(bytes.Buffer), nil
}

func (s *MemoryStorage) CloseEntry(entry io.ReadWriter) ([]byte, error) {
	buff, ok := entry.(*bytes.Buffer)
	if !ok {
		return nil, errors.New("MemoryStorage: Could not convert entry to buffer again")
	}
	return buff.Bytes(), nil
}

/* MMap Storage */

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

type MMapStorage struct {
	path string
}

func NewMMapStorage(path string) *MMapStorage {
	return &MMapStorage{path: path}
}

func (s *MMapStorage) Setup() error {
	s.path = path.Join("/tmp", "caddy-cache", randSeq(10))
	return os.MkdirAll(s.path, 0777)
}

func (s *MMapStorage) NewEntry(key string) (io.ReadWriter, error) {
	return os.OpenFile(path.Join(s.path, randSeq(15)), os.O_CREATE|os.O_RDWR, 0777)
}

func (s *MMapStorage) CloseEntry(entry io.ReadWriter) ([]byte, error) {
	file, ok := entry.(*os.File)
	if !ok {
		return nil, errors.New("MMapStorage: Could not convert entry file again")
	}
	info, err := file.Stat()
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	return syscall.Mmap(int(file.Fd()), 0, int(info.Size()), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
}

func (s *MMapStorage) Clean() error {
	// TODO clean folder
	return nil
}
