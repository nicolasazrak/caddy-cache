package cache

import (
	"io"
	"net/http"
	"time"
)

// HTTPCacheEntry saves the request response of an http request
type HTTPCacheEntry struct {
	isPublic   bool
	Expiration time.Time

	Request  *http.Request
	Response *Response
}

// Response holds the information about an upstream response
type Response struct {
	Code      int          // the HTTP response code from WriteHeader
	HeaderMap http.Header  // the HTTP response headers
	body      EntryStorage // The body saved as an EntryStorage
}

// NewHTTPCacheEntry creates a new CacheEntry for the given request
func NewHTTPCacheEntry(r *http.Request) *HTTPCacheEntry {
	return &HTTPCacheEntry{
		Request: r,
	}
}

// Clean removes the underlying file. It will block until
// There are no more subscribers. It's important to be sure
// that there will be no new subscribers or that will probably
// fail
func (e *HTTPCacheEntry) Clean() error {
	if e.Response.body == nil {
		return nil
	}

	return e.Response.body.Clean()
}

func (e *HTTPCacheEntry) OnFlush() {
	e.Response.body.Flush()
}

// OnWrite is called to notify when a write was made in the response
// To notify the subscribers
func (e *HTTPCacheEntry) OnWrite() {
	e.OnFlush()
}

// Close the entry meaning the the response has ended
func (e *HTTPCacheEntry) Close() error {
	if e.Response.body == nil {
		return nil
	}

	return e.Response.body.Close()
}

// GetBodyReader returns a io.ReadCloser that will contain
// the body content. Note that if calling Read returns 0
// does not mean the body has ended, there might be more content
// being fetched from upstream. So to know when the body has ended
// The client has to use Subscribe() channel
// After using it the client should first close() and then Unsubscribe()
func (e *HTTPCacheEntry) GetBodyReader() (io.ReadCloser, error) {
	return e.Response.body.GetReader()
}

func (e *HTTPCacheEntry) GetBodyWriter() io.Writer {
	return e.Response.body
}

func (e *HTTPCacheEntry) updateBodyWriter() error {
	if e.isPublic {
		storage, err := NewFileEntryStorage()
		if err != nil {
			return err
		}

		e.Response.body = storage
		return nil
	}

	e.Response.body, _ = NewBufferEntryStorage()
	return nil
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
