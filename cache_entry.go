package cache

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"
)

type HttpCacheEntry struct {
	isPublic   bool
	Expiration time.Time

	closed     bool
	closedLock *sync.RWMutex

	subscribers     []chan struct{}
	subscribersLock *sync.RWMutex

	Request  *http.Request
	Response *Response
}

type Response struct {
	Code      int         // the HTTP response code from WriteHeader
	HeaderMap http.Header // the HTTP response headers

	// Body content can be stored in a file if is cached
	// Or in the buffer if it's not and used privately
	// For the current request
	bodyFile   *os.File
	bodyBuffer *ConcurrentBuffer
}

func NewHTTPCacheEntry(r *http.Request) *HttpCacheEntry {
	return &HttpCacheEntry{
		Request:         r,
		subscribersLock: new(sync.RWMutex),
		closedLock:      new(sync.RWMutex),
	}
}

func (e *HttpCacheEntry) Subscribe() <-chan struct{} {
	newSubscriber := make(chan struct{})

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

func (e *HttpCacheEntry) notifySubscribers() {
	e.subscribersLock.RLock()
	defer e.subscribersLock.RUnlock()
	for _, subscriber := range e.subscribers {
		subscriber <- struct{}{}
	}
}

func (e *HttpCacheEntry) RemoveSubscriber(subscriber <-chan struct{}) {
	e.subscribersLock.Lock()
	defer e.subscribersLock.Unlock()
	for i, x := range e.subscribers {
		if x == subscriber {
			e.subscribers = append(e.subscribers[:i], e.subscribers[i+1:]...)
			break
		}
	}
}

func (e *HttpCacheEntry) OnFlush() {
	e.closedLock.RLock()
	defer e.closedLock.RUnlock()

	if e.isPublic {
		e.Response.bodyFile.Sync()
	}
	e.notifySubscribers()
}

func (e *HttpCacheEntry) OnWrite() {
	e.closedLock.RLock()
	defer e.closedLock.RUnlock()

	if e.isPublic {
		e.Response.bodyFile.Sync()
	}
	e.notifySubscribers()
}

func (e *HttpCacheEntry) Close() {
	e.closedLock.Lock()
	defer e.closedLock.Unlock()
	e.closed = true

	if e.isPublic {
		e.Response.bodyFile.Close()
	}

	e.subscribersLock.RLock()
	defer e.subscribersLock.RUnlock()
	for _, subscriber := range e.subscribers {
		close(subscriber)
	}
}

func (e *HttpCacheEntry) GetBodyReader() (io.ReadCloser, error) {
	if !e.isPublic {
		return ioutil.NopCloser(e.Response.bodyBuffer), nil
	}

	return os.Open(e.Response.bodyFile.Name())
}

func (e *HttpCacheEntry) GetBodyWriter() io.Writer {
	if e.isPublic {
		return e.Response.bodyFile
	}

	return e.Response.bodyBuffer
}

func (e *HttpCacheEntry) updateBodyWriter() error {
	if !e.isPublic {
		e.Response.bodyBuffer = new(ConcurrentBuffer)
		return nil
	}

	f, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	e.Response.bodyFile = f
	return err
}

func (e *HttpCacheEntry) UpdateResponse(response *Response) error {
	isPublic, expiration, err := getCacheableStatus(e.Request, response.Code, response.HeaderMap)
	if err != nil {
		return err
	}

	e.Response = response
	e.isPublic = isPublic
	e.Expiration = expiration

	err = e.updateBodyWriter()
	if err != nil {
		return err
	}

	return nil
}
