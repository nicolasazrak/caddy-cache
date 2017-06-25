package cache

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type HttpCacheEntry struct {
	isPublic    bool
	Expiration  time.Time
	closed      bool
	subscribers []chan struct{}
	Request     *http.Request
	Response    *Response
}

type Response struct {
	Code      int         // the HTTP response code from WriteHeader
	HeaderMap http.Header // the HTTP response headers

	// Body content can be stored in a file if is cached
	// Or in the buffer if it's not and used privately
	// For the current request
	bodyFile   *os.File
	bodyBuffer *bytes.Buffer
}

func (e *HttpCacheEntry) Subscribe() <-chan struct{} {
	newSubscriber := make(chan struct{})
	if e.closed {
		fmt.Println("There was a subscribe but the entry is already closed")
		close(newSubscriber)
		return newSubscriber
	}
	fmt.Println("Adding subscriber to an opened entry")
	e.subscribers = append(e.subscribers, newSubscriber)
	return newSubscriber
}

func (e *HttpCacheEntry) notifySubscribers() {
	// fmt.Println("Notifying subscribers of more content", len(e.subscribers))
	for _, subscriber := range e.subscribers {
		subscriber <- struct{}{}
	}
}

func (e *HttpCacheEntry) RemoveSubscriber(subscriber <-chan struct{}) {
	for i, x := range e.subscribers {
		if x == subscriber {
			e.subscribers = append(e.subscribers[:i], e.subscribers[i+1:]...)
			break
		}
	}
}

func (e *HttpCacheEntry) OnFlush() {
	if e.isPublic {
		e.Response.bodyFile.Sync()
	}
	e.notifySubscribers()
}

func (e *HttpCacheEntry) OnWrite() {
	if e.isPublic {
		e.Response.bodyFile.Sync()
	}
	e.notifySubscribers()
}

func (e *HttpCacheEntry) Close() {
	e.closed = true

	e.Response.bodyFile.Close()
	fmt.Println("Closing the entry", len(e.subscribers))
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
		return e.Response.bodyBuffer
	}

	return e.Response.bodyFile
}

func (e *HttpCacheEntry) updateBodyWriter() error {
	if !e.isPublic {
		e.Response.bodyBuffer = new(bytes.Buffer)
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

	fmt.Println("Cacheable: ", isPublic, expiration)
	e.Response = response
	e.isPublic = true
	// e.isPublic = isPublic
	e.Expiration = expiration

	err = e.updateBodyWriter()
	if err != nil {
		return err
	}

	return nil
}
