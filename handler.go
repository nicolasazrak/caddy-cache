package cache

import (
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type Request struct {
	HeaderMap http.Header // Headers are the only useful information
}

type Response struct {
	Code      int // the HTTP response code from WriteHeader
	Body      []byte
	HeaderMap http.Header // the HTTP response headers
}

type HttpCacheEntry struct {
	// IsCached might not be the better name
	// It means that the response can be sent to the client
	// Because not only public responses are stored, private too.
	// So if this is false, it means a request to upstream is needed
	IsCached   bool
	Expiration time.Time
	Request    *Request
	Response   *Response
}

type CacheHandler struct {
	Config  *Config
	Storage *MemoryStorage
	Next    httpserver.Handler
}

func respond(response *Response, w http.ResponseWriter) {
	for k, values := range response.HeaderMap {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(response.Code)
	w.Write(response.Body)
}

/**
 * Builds the cache key
 */
func getKey(r *http.Request) string {
	key := r.Method + " " + r.Host + r.URL.Path

	q := r.URL.Query().Encode()
	if len(q) > 0 {
		key += "?" + q
	}

	return key
}

/**
 * Returns a function that given a previous response returns if it matches the current response
 */
func matchesRequest(r *http.Request) func(*HttpCacheEntry) bool {
	return func(entry *HttpCacheEntry) bool {
		// TODO match getKeys()
		// It is always called with same key values
		// But checking it is better

		vary, hasVary := entry.Response.HeaderMap["Vary"]
		if !hasVary {
			return true
		}

		for _, searchedHeader := range strings.Split(vary[0], ",") {
			searchedHeader = strings.TrimSpace(searchedHeader)
			if !reflect.DeepEqual(entry.Request.HeaderMap[searchedHeader], r.Header[searchedHeader]) {
				return false
			}
		}
		return true
	}
}

func (h *CacheHandler) AddStatusHeaderIfConfigured(w http.ResponseWriter, status string) {
	if h.Config.StatusHeader != "" {
		w.Header().Add(h.Config.StatusHeader, status)
	}
}

/**
* This prevents storing status header in cache.
* Otherwise the status cache will be sent twice for cached results
 */
func (h *CacheHandler) RemoveStatusHeaderIfConfigured(headers http.Header) http.Header {
	if h.Config.StatusHeader != "" {
		delete(headers, h.Config.StatusHeader)
	}
	return headers
}

func (handler CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	if !shouldUseCache(r) {
		handler.AddStatusHeaderIfConfigured(w, "skip")
		return handler.Next.ServeHTTP(w, r)
	}

	returnedStatusCode := 500 // If this is not updated means there was an error
	err := handler.Storage.GetOrLock(getKey(r), matchesRequest(r), func(previous *HttpCacheEntry) error {

		if previous == nil || !previous.IsCached {
			rec := NewStreamedRecorder(w)

			handler.AddStatusHeaderIfConfigured(w, "miss")

			_, err := handler.Next.ServeHTTP(rec, r)
			if err != nil {
				return err
			}

			isCacheable, expirationTime, err := getCacheableStatus(r, rec, handler.Config)
			if err != nil {
				return err
			}

			// This is useless if request is not cacheable, but the response is already recorded
			// The next todo thing is avoid this
			storedBody := rec.Body.Bytes()

			if !isCacheable {
				// Non cacheable responses are stored to avoid locking
				// But body is not useful
				storedBody = []byte{}

				// Re validate locking on 60 seconds
				expirationTime = time.Now().UTC().Add(time.Duration(1) * time.Hour)
			}

			// Build the cache entry
			response := &HttpCacheEntry{
				IsCached:   isCacheable,
				Expiration: expirationTime,
				Request: &Request{
					HeaderMap: r.Header,
				},
				Response: &Response{
					Body:      storedBody,
					HeaderMap: handler.RemoveStatusHeaderIfConfigured(rec.Result().Header),
					Code:      rec.Result().StatusCode,
				},
			}

			// Set instead of push to prevent storing a lot responses unused
			err = handler.Storage.Push(getKey(r), response)
			if err != nil {
				return err
			}

			returnedStatusCode = rec.Result().StatusCode
			return nil
		} else {
			handler.AddStatusHeaderIfConfigured(w, "hit")
			respond(previous.Response, w)
			returnedStatusCode = previous.Response.Code
			return nil
		}
	})
	return returnedStatusCode, err
}
