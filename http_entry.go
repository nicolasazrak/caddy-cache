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
	Expiration time.Time
	Request    *Request
	Response   *Response
}

// IsCached might not be the better name
// It means that the response can be sent to the client
// Because not only public responses are stored, private too.
// So if this is false, it means a request to upstream is needed
func (entry *HttpCacheEntry) IsCached() bool {
	// TODO what happens with HEAD requests ??
	return entry.Response != nil && entry.Response.Body != nil
}

func (entry *HttpCacheEntry) Clear() error {
	// TODO why Response can be nil?
	if entry.Response != nil && entry.Response.Body != nil {
		return entry.Response.Body.Clear()
	}
	return nil
}
