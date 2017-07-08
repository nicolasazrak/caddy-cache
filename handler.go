package cache

import (
	"net/http"
	"time"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

// Handler is the main cache middleware
type Handler struct {
	// Handler configuration
	Config *Config

	// A map with URL -> List of cached entries
	Entries map[string][]*HTTPCacheEntry

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

func NewHandler(Next httpserver.Handler) *Handler {
	return &Handler{
		Entries:  map[string][]*HTTPCacheEntry{},
		URLLocks: NewURLLock(),
		Next:     Next,
	}
}

/* Entries */

func (handler *Handler) saveEntry(updatedEntry *HTTPCacheEntry) {
	key := getKey(updatedEntry.Request)

	handler.scheduleCleanEntry(updatedEntry)

	for i, previousEntry := range handler.Entries[key] {
		if matchesVary(updatedEntry.Request, previousEntry.Response) {
			go previousEntry.Clean()
			handler.Entries[key][i] = updatedEntry
			return
		}
	}

	handler.Entries[key] = append(handler.Entries[key], updatedEntry)
}

func (handler *Handler) getEntry(r *http.Request) (*HTTPCacheEntry, bool) {
	// TODO fix data race
	// Why is it happening? Gorace detects a race between this and cleanEntry
	previousEntries, exists := handler.Entries[getKey(r)]

	if !exists {
		return nil, false
	}

	for _, entry := range previousEntries {
		if matchesVary(r, entry.Response) {
			return entry, true
		}
	}

	return nil, false
}

func (handler *Handler) scheduleCleanEntry(entry *HTTPCacheEntry) {
	go func(entry *HTTPCacheEntry) {
		time.Sleep(entry.expiration.Sub(time.Now().UTC()))
		handler.cleanEntry(entry)
	}(entry)
}

func (handler *Handler) cleanEntry(entry *HTTPCacheEntry) {
	key := getKey(entry.Request)

	lock := handler.URLLocks.Adquire(key)
	defer lock.Unlock()

	for i, otherEntry := range handler.Entries[getKey(entry.Request)] {
		if entry == otherEntry {
			// TODO fix data race
			// Why is it happening?
			handler.Entries[key] = append(handler.Entries[key][:i], handler.Entries[key][i+1:]...)
			entry.Clean()
			return
		}
	}
}

/* Responses */

func (handler *Handler) addStatusHeaderIfConfigured(w http.ResponseWriter, status string) {
	w.Header().Add("X-Cache-status", status)
}

func (handler *Handler) respond(w http.ResponseWriter, entry *HTTPCacheEntry, cacheStatus string) (int, error) {
	handler.addStatusHeaderIfConfigured(w, cacheStatus)
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
	response := NewResponse()

	var err error

	go func(req *http.Request, response *Response) {
		statusCode, upstreamError := handler.Next.ServeHTTP(response, req)
		err = upstreamError
		response.WriteHeader(statusCode) // If headers were not set this will replace them
		response.WaitBody()
		response.Close()
	}(req, response)

	response.WaitHeaders()
	return NewHTTPCacheEntry(req, response, handler.Config), err
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	if !shouldUseCache(r) {
		handler.addStatusHeaderIfConfigured(w, cacheBypass)
		return handler.Next.ServeHTTP(w, r)
	}

	lock := handler.URLLocks.Adquire(getKey(r))

	// Lookup correct entry
	previousEntry, exists := handler.getEntry(r)

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
			lock := handler.URLLocks.Adquire(getKey(r))
			handler.saveEntry(entry)
			lock.Unlock()
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
			return 500, err
		}
	}
	handler.saveEntry(entry)
	lock.Unlock()
	return handler.respond(w, entry, cacheMiss)
}
