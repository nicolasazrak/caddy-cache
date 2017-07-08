package cache

import (
	"io"
	"net/http"
	"time"
)

// HTTPCacheEntry saves the request response of an http request
type HTTPCacheEntry struct {
	isPublic   bool
	expiration time.Time

	Request  *http.Request
	Response *Response
}

// NewHTTPCacheEntry creates a new HTTPCacheEntry for the given request
// and creates a new storage for the body in case the response is public
func NewHTTPCacheEntry(request *http.Request, response *Response, config *Config) *HTTPCacheEntry {
	isPublic, expiration := getCacheableStatus(request, response, config)

	return &HTTPCacheEntry{
		isPublic:   isPublic,
		expiration: expiration,
		Request:    request,
		Response:   response,
	}
}

func (e *HTTPCacheEntry) Clean() error {
	return e.Response.Clean()
}

func (e *HTTPCacheEntry) writePublicResponse(w http.ResponseWriter) error {
	reader, err := e.Response.body.GetReader()
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = io.Copy(w, reader)
	return err
}

func (e *HTTPCacheEntry) writePrivateResponse(w http.ResponseWriter) error {
	e.Response.SetBody(WrapResponseWriter(w))
	e.Response.WaitClose()
	return nil
}

func (e *HTTPCacheEntry) WriteBodyTo(w http.ResponseWriter) error {
	if !e.isPublic {
		return e.writePrivateResponse(w)
	}
	return e.writePublicResponse(w)
}

func (e *HTTPCacheEntry) setStorage() error {
	storage, err := NewFileStorage()

	// Set the storage even if it is nil to continue and stop the upstream request
	e.Response.SetBody(storage)

	return err
}
