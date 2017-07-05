package cache

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"sync"
)

// ResponseStorage represents a possible storage for the entry
type ResponseStorage interface {
	io.Writer
	io.Closer
	Clean() error
	Flush() error
	GetReader() (io.ReadCloser, error)
}

/////////////////////////////////////////////////////////////////////////////////////////////////////

// BufferStorage saves the content into a buffer
type BufferStorage struct {
	b            *bytes.Buffer
	m            *sync.RWMutex
	subscription *Subscription
}

// NewBufferEntryStorage creates a new emptyStorage
func NewBufferEntryStorage() (ResponseStorage, error) {
	return &BufferStorage{
		subscription: NewSubscription(),
		m:            new(sync.RWMutex),
		b:            new(bytes.Buffer),
	}, nil
}

func (b *BufferStorage) Write(p []byte) (n int, err error) {
	b.m.Lock()
	n, err = b.b.Write(p)
	b.m.Unlock()
	b.subscription.NotifyAll()
	return n, err
}

// Flush does nothing in a buffer
func (b *BufferStorage) Flush() error {
	return nil
}

// Clean does nothing in the buffer it will be garbage collected eventually
func (b *BufferStorage) Clean() error {
	b.subscription.WaitAll()
	return nil
}

// Close the storage entry
func (b *BufferStorage) Close() error {
	b.subscription.Close()
	return nil
}

// GetReader returns the same buffer
func (b *BufferStorage) GetReader() (io.ReadCloser, error) {
	return &StorageReader{
		content:      &BufferReader{b: b.b, m: b.m},
		subscription: b.subscription.NewSubscriber(),
		unsubscribe:  b.subscription.RemoveSubscriber,
	}, nil
}

// BufferReader reads the buffer using the writing lock to prevent race conditions
type BufferReader struct {
	b *bytes.Buffer
	m *sync.RWMutex
}

func (b *BufferReader) Read(p []byte) (n int, err error) {
	b.m.RLock()
	defer b.m.RUnlock()
	return b.b.Read(p)
}

func (b *BufferReader) Close() error {
	return nil
}

/////////////////////////////////////////////////////////////////////////////////////////////////////

// FileStorage saves the content into a file
type FileStorage struct {
	file         *os.File
	subscription *Subscription
}

// NewFileEntryStorage creates a new temp file that will be used as a the storage of the cache entry
func NewFileEntryStorage() (ResponseStorage, error) {
	file, err := ioutil.TempFile("", "caddy-cache-")
	if err != nil {
		return nil, err
	}
	return &FileStorage{
		file:         file,
		subscription: NewSubscription(),
	}, nil
}

func (f *FileStorage) Write(p []byte) (n int, err error) {
	defer f.subscription.NotifyAll()
	return f.file.Write(p)
}

// Flush syncs the underlying file
func (f *FileStorage) Flush() error {
	defer f.subscription.NotifyAll()
	return f.file.Sync()
}

// Clean removes the file
func (f *FileStorage) Clean() error {
	f.subscription.WaitAll() // Wait until every subscriber ends waiting every result
	return os.Remove(f.file.Name())
}

// Close the underlying file
func (f *FileStorage) Close() error {
	f.subscription.Close()
	return f.file.Close()
}

// GetReader returns a new file descriptor to the same file
func (f *FileStorage) GetReader() (io.ReadCloser, error) {
	newFile, err := os.Open(f.file.Name())
	if err != nil {
		return nil, err
	}
	return &StorageReader{
		content:      newFile,
		subscription: f.subscription.NewSubscriber(),
		unsubscribe:  f.subscription.RemoveSubscriber,
	}, nil
}

/////////////////////////////////////////////////////////////////////////////////////////////////////

// StorageReader is the common code to read the storages until the subscription channel is closed
type StorageReader struct {
	subscription <-chan struct{}
	content      io.ReadCloser
	unsubscribe  func(<-chan struct{})
}

func (r *StorageReader) Read(p []byte) (n int, err error) {
	for range r.subscription {
		n, err := r.content.Read(p)
		if err != io.EOF {
			return n, err
		}
	}

	return r.content.Read(p)
}

// Close closes the underlying storage
func (r *StorageReader) Close() error {
	err := r.content.Close()
	r.unsubscribe(r.subscription)
	return err
}

/////////////////////////////////////////////////////////////////////////////////////////////////////

type Subscription struct {
	closed            bool
	closedLock        *sync.RWMutex
	subscribers       []chan struct{}
	noSubscribersChan chan struct{}
	subscribersLock   *sync.RWMutex
}

func NewSubscription() *Subscription {
	return &Subscription{
		closedLock:        new(sync.RWMutex),
		subscribersLock:   new(sync.RWMutex),
		noSubscribersChan: make(chan struct{}, 1),
	}
}

func (s *Subscription) NewSubscriber() <-chan struct{} {
	s.closedLock.Lock()
	defer s.closedLock.Unlock()
	if s.closed {
		subscription := make(chan struct{})
		close(subscription)
		return subscription
	}

	s.subscribersLock.Lock()
	defer s.subscribersLock.Unlock()
	subscription := make(chan struct{}, 1)
	s.subscribers = append(s.subscribers, subscription)
	return subscription
}

func (s *Subscription) RemoveSubscriber(subscriber <-chan struct{}) {
	s.subscribersLock.Lock()
	defer s.subscribersLock.Unlock()
	for i, x := range s.subscribers {
		if x == subscriber {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			break
		}
	}

	noSubscribers := len(s.subscribers) == 0
	if noSubscribers {
		select {
		case s.noSubscribersChan <- struct{}{}:
		default:
		}
	}
}

func (s *Subscription) Close() {
	s.closedLock.Lock()
	defer s.closedLock.Unlock()
	if s.closed {
		return
	}

	s.closed = true
	s.subscribersLock.RLock()
	defer s.subscribersLock.RUnlock()
	for _, subscriber := range s.subscribers {
		close(subscriber)
	}
}

func (s *Subscription) NotifyAll() {
	s.subscribersLock.RLock()
	defer s.subscribersLock.RUnlock()
	for _, subscriber := range s.subscribers {
		subscriber <- struct{}{}
	}
}

func (s *Subscription) hasSubscribers() bool {
	s.subscribersLock.RLock()
	defer s.subscribersLock.RUnlock()
	return len(s.subscribers) != 0
}

// WaitAll waits until are subscribers ends
func (s *Subscription) WaitAll() {
	if !s.hasSubscribers() {
		return
	}
	for range s.noSubscribersChan {
		if !s.hasSubscribers() {
			return
		}
	}
}
