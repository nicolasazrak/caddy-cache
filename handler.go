package cache

import (
	"net/http"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

// Handler is the main cache middleware
type Handler struct {
	// Handler configuration
	Config *Config

	// Cache is where entries are stored
	Cache *HTTPCache

	// Next handler
	Next httpserver.Handler

	// Handles locking for different URLs
	URLLocks *URLLock
}

const (
	cacheHit    = "hit"
	cacheMiss   = "miss"
	cacheSkip   = "skip"
	cacheBypass = "bypass"
)

func getKey(r *http.Request) string {
	key := r.Method + " " + r.Host + r.URL.Path

	q := r.URL.Query().Encode()
	if len(q) > 0 {
		key += "?" + q
	}

	return key
}

// NewHandler creates a new Handler using Next middleware
func NewHandler(Next httpserver.Handler) *Handler {
	return &Handler{
		Cache:    NewHTTPCache(),
		URLLocks: NewURLLock(),
		Next:     Next,
	}
}

/* Responses */

func (handler *Handler) addStatusHeaderIfConfigured(w http.ResponseWriter, status string) {
	w.Header().Add("X-Cache-status", status)
}

func (handler *Handler) respond(w http.ResponseWriter, entry *HTTPCacheEntry, cacheStatus string) (int, error) {
	handler.addStatusHeaderIfConfigured(w, cacheStatus)

	// Send entry headers
	for k, values := range entry.Response.HeaderMap {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(entry.Response.Code)

	err := entry.WriteBodyTo(w)

	return entry.Response.Code, err
}

/* Handler */

func shouldUseCache(req *http.Request) bool {
	// TODO Add more logic like get params, ?nocache=true

	if req.Method != "GET" && req.Method != "HEAD" {
		// Only cache Get and head request
		return false
	}

	// Range responses still not supported
	if req.Header.Get("accept-ranges") != "" {
		return false
	}

	return true
}

func (handler *Handler) fetchUpstream(req *http.Request) (*HTTPCacheEntry, error) {
	// Create a new empty response
	response := NewResponse()

	var err error

	// Do the upstream fetching in background
	go func(req *http.Request, response *Response) {
		statusCode, upstreamError := handler.Next.ServeHTTP(response, req)
		err = upstreamError
		response.WriteHeader(statusCode) // If headers were not set this will replace them

		// Wait the response body to be set.
		// If it is private it will be the original http.ResponseWriter
		// It is required to wait the body to prevent closing the response
		// before the body was set. If that happens the body will
		// stay locked waiting the response to be closed
		response.WaitBody()
		response.Close()
	}(req, response)

	// Wait headers to de sent
	response.WaitHeaders()

	// Create a new CacheEntry
	return NewHTTPCacheEntry(getKey(req), req, response, handler.Config), err
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	if !shouldUseCache(r) {
		handler.addStatusHeaderIfConfigured(w, cacheBypass)
		return handler.Next.ServeHTTP(w, r)
	}

	lock := handler.URLLocks.Adquire(getKey(r))

	// Lookup correct entry
	previousEntry, exists := handler.Cache.Get(r)

	// First case: CACHE HIT
	// The response exists in cache and is public
	// It should be served as saved
	if exists && previousEntry.isPublic {
		lock.Unlock()
		return handler.respond(w, previousEntry, cacheHit)
	}

	// Second case: CACHE SKIP
	// The response is in cache but it is not public
	// It should NOT be served from cache
	// It should be fetched from upstream and check the new headers
	// To check if the new response changes to public
	if exists && !previousEntry.isPublic {
		lock.Unlock()
		entry, err := handler.fetchUpstream(r)
		if err != nil {
			return entry.Response.Code, err
		}

		// Case when response was private but now is public
		if entry.isPublic {
			err := entry.setStorage()
			if err != nil {
				return 500, err
			}

			handler.Cache.Put(r, entry)
			return handler.respond(w, entry, cacheMiss)
		}

		return handler.respond(w, entry, cacheSkip)
	}

	// Third case: CACHE MISS
	// The response is not in cache
	// It should be fetched from upstream and save it in cache
	entry, err := handler.fetchUpstream(r)
	if err != nil {
		lock.Unlock()
		return entry.Response.Code, err
	}

	// Entry is always saved, even if it is not public
	// This is to release the URL lock.
	if entry.isPublic {
		err := entry.setStorage()
		if err != nil {
			lock.Unlock()
			return 500, err
		}
	}

	handler.Cache.Put(r, entry)
	lock.Unlock()
	return handler.respond(w, entry, cacheMiss)
}
