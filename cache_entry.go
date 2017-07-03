package cache

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"
)

// HTTPCacheEntry saves the request response of an http request
type HTTPCacheEntry struct {
	isPublic   bool
	Expiration time.Time

	closed     bool
	closedLock *sync.RWMutex

	noSubscribersChan chan struct{} // Chan to inform that there are no new subscribers, used for cleanup
	subscribers       []chan struct{}
	subscribersLock   *sync.RWMutex

	Request  *http.Request
	Response *Response
}

// Response holds the information about an upstream response
type Response struct {
	Code      int         // the HTTP response code from WriteHeader
	HeaderMap http.Header // the HTTP response headers

	// Body content can be stored in a file if is cached
	// Or in the buffer if it's not and used privately
	// For the current request
	bodyFile   *os.File
	bodyBuffer *ConcurrentBuffer
}

// NewHTTPCacheEntry creates a new CacheEntry for the given request
func NewHTTPCacheEntry(r *http.Request) *HTTPCacheEntry {
	return &HTTPCacheEntry{
		Request:           r,
		subscribersLock:   new(sync.RWMutex),
		closedLock:        new(sync.RWMutex),
		noSubscribersChan: make(chan struct{}, 1),
	}
}

// Subscribe returns a channel that will emit an empty struct
// each time there is a new content in body reader
func (e *HTTPCacheEntry) Subscribe() <-chan struct{} {
	newSubscriber := make(chan struct{}, 1)

	e.closedLock.Lock()
	defer e.closedLock.Unlock()

	if e.closed {
		close(newSubscriber)
		return newSubscriber
	}

	e.subscribersLock.Lock()
	defer e.subscribersLock.Unlock()

	e.subscribers = append(e.subscribers, newSubscriber)
	return newSubscriber
}

func (e *HTTPCacheEntry) notifySubscribers() {
	e.subscribersLock.RLock()
	defer e.subscribersLock.RUnlock()
	for _, subscriber := range e.subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

// RemoveSubscriber removes a subscription
// It is important to know that. Otherwise the entry cannot be cleaned
func (e *HTTPCacheEntry) RemoveSubscriber(subscriber <-chan struct{}) {
	e.subscribersLock.Lock()
	defer e.subscribersLock.Unlock()
	for i, x := range e.subscribers {
		if x == subscriber {
			e.subscribers = append(e.subscribers[:i], e.subscribers[i+1:]...)
			break
		}
	}

	if len(e.subscribers) == 0 {
		// Send a notification in a non-blocking way
		select {
		case e.noSubscribersChan <- struct{}{}:
		default:
		}
	}
}

// Clean removes the underlying file. It will block until
// There are no more subscribers. It's important to be sure
// that there will be no new subscribers or that will probably
// fail
func (e *HTTPCacheEntry) Clean() error {
	if e.Response.bodyFile == nil {
		return nil
	}

	// Wait until no subscribers
	for range e.noSubscribersChan {
		e.subscribersLock.RLock()
		noSubscribers := len(e.subscribers) == 0
		e.subscribersLock.RUnlock()

		if noSubscribers {
			break
		}
	}

	return os.Remove(e.Response.bodyFile.Name())
}

func (e *HTTPCacheEntry) OnFlush() {
	e.closedLock.RLock()
	defer e.closedLock.RUnlock()

	if e.isPublic {
		e.Response.bodyFile.Sync()
	}
	e.notifySubscribers()
}

// OnWrite is called to notify when a write was made in the response
// To notify the subscribers
func (e *HTTPCacheEntry) OnWrite() {
	e.OnFlush()
}

// Close the entry meaning the the response has ended
func (e *HTTPCacheEntry) Close() error {
	e.closedLock.Lock()
	defer e.closedLock.Unlock()
	e.closed = true

	e.subscribersLock.RLock()
	defer e.subscribersLock.RUnlock()
	for _, subscriber := range e.subscribers {
		close(subscriber)
	}

	if e.isPublic {
		err := e.Response.bodyFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// GetBodyReader returns a io.ReadCloser that will contain
// the body content. Note that if calling Read returns 0
// does not mean the body has ended, there might be more content
// being fetched from upstream. So to know when the body has ended
// The client has to use Subscribe() channel
// After using it the client should first close() and then Unsubscribe()
func (e *HTTPCacheEntry) GetBodyReader() (io.ReadCloser, error) {
	if !e.isPublic {
		return ioutil.NopCloser(e.Response.bodyBuffer), nil
	}

	return os.Open(e.Response.bodyFile.Name())
}

func (e *HTTPCacheEntry) GetBodyWriter() io.Writer {
	if e.isPublic {
		return e.Response.bodyFile
	}

	return e.Response.bodyBuffer
}

func (e *HTTPCacheEntry) updateBodyWriter() error {
	if !e.isPublic {
		e.Response.bodyBuffer = new(ConcurrentBuffer)
		return nil
	}

	f, err := ioutil.TempFile("", "caddy-cache-")
	if err != nil {
		return err
	}
	e.Response.bodyFile = f
	return err
}

// UpdateResponse saves the response and updates
// the isPublic, expiration values and more importantly
// The bodyWriter
func (e *HTTPCacheEntry) UpdateResponse(response *Response) error {
	isPublic, expiration, err := getCacheableStatus(e.Request, response.Code, response.HeaderMap)
	if err != nil {
		return err
	}

	e.Response = response
	e.isPublic = isPublic
	e.Expiration = expiration

	return e.updateBodyWriter()
}
