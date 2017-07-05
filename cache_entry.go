package cache

import (
	"net/http"
	"time"
)

// HTTPCacheEntry saves the request response of an http request
type HTTPCacheEntry struct {
	Request  *http.Request
	Response *Response
}

// NewHTTPCacheEntry creates a new CacheEntry for the given request
func NewHTTPCacheEntry(request *http.Request, response *Response) *HTTPCacheEntry {
	return &HTTPCacheEntry{
		Request:  request,
		Response: response,
	}
}

func (e *HTTPCacheEntry) Clean() error {
	return e.Response.Clean()
}

func (e *HTTPCacheEntry) Close() error {
	return e.Response.Close()
}

func (e *HTTPCacheEntry) Expiration() time.Time {
	return e.Response.expiration
}

func (e *HTTPCacheEntry) IsPublic() bool {
	return e.Response.isPublic
}
