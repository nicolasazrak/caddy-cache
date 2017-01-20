package cache

import (
	"fmt"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type CacheHandler struct {
	Config *Config
	Cache  *Cache
	Next   httpserver.Handler
}

func respond(response *Response, w http.ResponseWriter) {
	for k, values := range response.HeaderMap {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(response.Code)
	w.Write(response.Body.Bytes())
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
		w.Header().Set(h.Config.StatusHeader, status)
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

func (handler *CacheHandler) HandleCachedResponse(w http.ResponseWriter, r *http.Request, previous *HttpCacheEntry) int {
	handler.AddStatusHeaderIfConfigured(w, "hit")
	respond(previous.Response, w)
	return previous.Response.Code
}

func (handler *CacheHandler) HandleNonCachedResponse(w http.ResponseWriter, r *http.Request) (*HttpCacheEntry, error) {
	key := getKey(r)
	rec := NewStreamedRecorder(w)

	// Build the cache entry
	entry := &HttpCacheEntry{
		Expiration: time.Now().UTC().Add(time.Duration(1) * time.Hour),
		Request:    &Request{HeaderMap: r.Header},
		Response:   nil,
	}

	// Create a callback on response recorder
	// So as soon as the first byte is sent, check the headers
	// If the response is cacheable, a new entry will be created
	// And the response will be saved. In case the mmap storage is
	// being used, the response will be saved to a file
	rec.SetFirstWriteListener(func(Code int, Header http.Header) error {
		isCacheable, expirationTime, err := getCacheableStatus(r, Code, Header, handler.Config)
		if err != nil {
			// getCacheableStatus may return an error when it fails to parse
			// Some header, but it is not be a problem here.
			// Just ignore it and don't cache that response.
			return nil
		}

		// If it's not cacheable do nothing
		if !isCacheable {
			return nil
		}

		// Update the expiration value
		entry.Expiration = expirationTime

		// Create the new entry, potentially creating a new file in disk
		writer, err := handler.Cache.NewContent(key)
		if err != nil {
			fmt.Println("Ups", err)
			panic(err)
		}

		// Update the body writer, next Writes will go to the created writer
		rec.UpdateBodyWriter(writer)
		return nil
	})

	// Send the status header and server the request from upstream
	handler.AddStatusHeaderIfConfigured(w, "miss")
	_, err := handler.Next.ServeHTTP(rec, r)
	if err != nil {
		return nil, err
	}

	result, Body := rec.Result()
	entry.Response = &Response{
		HeaderMap: handler.RemoveStatusHeaderIfConfigured(result.Header),
		Code:      result.StatusCode,
	}

	// If the body was recorded, close the body and update the entry
	if Body != nil {
		Body.Close()
		entry.Response.Body = Body
	}

	return entry, nil
}

func (handler CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	if !shouldUseCache(r) {
		handler.AddStatusHeaderIfConfigured(w, "skip")
		return handler.Next.ServeHTTP(w, r)
	}

	returnedStatusCode := http.StatusInternalServerError // If this is not updated means there was an error
	err := handler.Cache.GetOrSet(getKey(r), matchesRequest(r), func(previous *HttpCacheEntry) (*HttpCacheEntry, error) {
		if previous == nil || !previous.IsCached() {
			newEntry, err := handler.HandleNonCachedResponse(w, r)
			if err != nil {
				return nil, err
			}
			returnedStatusCode = newEntry.Response.Code
			return newEntry, nil
		}

		returnedStatusCode = handler.HandleCachedResponse(w, r, previous)
		return nil, nil
	})
	return returnedStatusCode, err
}
