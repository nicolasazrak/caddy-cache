package cache

import (
	"io"
	"net/http"
	"time"

	"github.com/nicolasazrak/caddy-cache/storage"
)

// HTTPCacheEntry saves the request response of an http request
type HTTPCacheEntry struct {
	isPublic   bool
	expiration time.Time
	key        string

	Request  *http.Request
	Response *Response
}

// NewHTTPCacheEntry creates a new HTTPCacheEntry for the given request and response
// and it also calculates if the response is public
func NewHTTPCacheEntry(key string, request *http.Request, response *Response, config *Config) *HTTPCacheEntry {
	isPublic, expiration := getCacheableStatus(request, response, config)

	return &HTTPCacheEntry{
		key:        key,
		isPublic:   isPublic,
		expiration: expiration,
		Request:    request,
		Response:   response,
	}
}

func (e *HTTPCacheEntry) Key() string {
	return e.key
}

// Clean removes the response if it has an associated file
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
	e.Response.SetBody(storage.WrapResponseWriter(w))
	e.Response.WaitClose()
	return nil
}

// WriteBodyTo sends the body to the http.ResponseWritter
func (e *HTTPCacheEntry) WriteBodyTo(w http.ResponseWriter) error {
	if !e.isPublic {
		return e.writePrivateResponse(w)
	}
	return e.writePublicResponse(w)
}

func (e *HTTPCacheEntry) setStorage(config *Config) error {
	storage, err := storage.NewFileStorage(config.Path)

	// Set the storage even if it is nil to continue and stop the upstream request
	e.Response.SetBody(storage)

	return err
}

// Fresh returns if the entry is still fresh
func (e *HTTPCacheEntry) Fresh() bool {
	return e.expiration.After(time.Now())
}
