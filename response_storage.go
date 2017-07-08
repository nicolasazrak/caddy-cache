package cache

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
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

// NoStorage writes the content directly into the ResponseWriter
// TODO remove this
type NoStorage struct {
	w http.ResponseWriter
}

// WrapResponseWriter wraps an http.ResponseWriter and gives it
// the ResponseStorage interface
func WrapResponseWriter(w http.ResponseWriter) ResponseStorage {
	return &NoStorage{
		w: w,
	}
}

func (b *NoStorage) Write(p []byte) (n int, err error) {
	return b.w.Write(p)
}

// Flush does nothing in a buffer
func (b *NoStorage) Flush() error {
	if f, ok := b.w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

// Clean does nothing in the buffer it will be garbage collected eventually
func (b *NoStorage) Clean() error {
	return nil
}

// Close the storage entry
func (b *NoStorage) Close() error {
	return nil
}

// GetReader returns the same buffer
func (b *NoStorage) GetReader() (io.ReadCloser, error) {
	return nil, errors.New("Private responses are no redable")
}

/////////////////////////////////////////////////////////////////////////////////////////////////////

// FileStorage saves the content into a file
type FileStorage struct {
	file         *os.File
	subscription *Subscription
}

// NewFileStorage creates a new temp file that will be used as a the storage of the cache entry
func NewFileStorage(path string) (ResponseStorage, error) {
	file, err := ioutil.TempFile(path, "caddy-cache-")
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
