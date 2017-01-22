package cache

import (
	"net/http"
	"time"
)

type Request struct {
	HeaderMap http.Header // Headers are the only useful information
}

type Response struct {
	Code      int // the HTTP response code from WriteHeader
	Body      StorageContent
	HeaderMap http.Header // the HTTP response headers
}

type HttpCacheEntry struct {
	isPublic   bool
	Expiration time.Time
	Request    *Request
	Response   *Response
}

func (entry *HttpCacheEntry) Clear() error {
	// TODO why Response can be nil?
	if entry.Response != nil && entry.Response.Body != nil {
		return entry.Response.Body.Clear()
	}
	return nil
}
