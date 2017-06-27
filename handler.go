package cache

import (
	"fmt"
	"io"
	"net/http"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

// Handler is the main cache middleware
type Handler struct {
	// Handler configuration
	Config *Config

	// A map with URL -> List of cached entries
	Entries map[string][]*HttpCacheEntry

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
	return r.URL.Path
}

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

func (handler *Handler) addStatusHeaderIfConfigured(w http.ResponseWriter, status string) {
	w.Header().Add("X-Cache-status", status)
}

func (handler *Handler) respond(w http.ResponseWriter, entry *HttpCacheEntry, cacheStatus string) (int, error) {
	body, err := entry.GetBodyReader()
	if err != nil {
		fmt.Println(err)
		return 500, err
	}
	defer body.Close()

	handler.addStatusHeaderIfConfigured(w, cacheStatus)
	for k, values := range entry.Response.HeaderMap {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(entry.Response.Code)

	moreContentChan := entry.Subscribe()
	defer entry.RemoveSubscriber(moreContentChan)

	io.Copy(w, body)
	for range moreContentChan {
		io.Copy(w, body)
	}
	io.Copy(w, body)

	return entry.Response.Code, nil
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	if !shouldUseCache(r) {
		handler.addStatusHeaderIfConfigured(w, cacheBypass)
		return handler.Next.ServeHTTP(w, r)
	}

	lock := handler.URLLocks.Adquire(getKey(r))

	// Lookup correct entry
	previousEntry, exists := handler.Entries[getKey(r)]

	// First case: CACHE HIT
	// The response exists in cache and is public
	// It should be served as saved
	if exists && previousEntry[0].isPublic {
		lock.Unlock()
		return handler.respond(w, previousEntry[0], cacheHit)
	}

	// Second case: CACHE SKIP
	// The response is in cache but it is not public
	// It should NOT be served from cache
	// It should be fetched from upstream and check the new headers
	// To check if the new response changes to public
	if exists && !previousEntry[0].isPublic {
		lock.Unlock()
		newResult := <-FetchUpstream(handler.Next, r)
		if newResult.err != nil {
			fmt.Println(newResult.err)
			lock.Unlock()
			return 500, newResult.err
		}
		newEntry := newResult.entry
		handler.respond(w, newEntry, cacheSkip)

		// Case when response was private but now is public
		if newEntry.isPublic {
			lock := handler.URLLocks.Adquire(getKey(r))
			// TODO check if it was not already replaced
			handler.Entries[getKey(r)][0] = newEntry
			lock.Unlock()
		}

		return newEntry.Response.Code, nil
	}

	// Third case: CACHE MISS
	// The response is not in cache
	// It should be fetched from upstream and save it in cache
	if !exists {
		newResult := <-FetchUpstream(handler.Next, r)
		if newResult.err != nil {
			fmt.Println(newResult.err)
			lock.Unlock()
			return 500, newResult.err
		}
		newEntry := newResult.entry
		handler.respond(w, newEntry, cacheMiss)

		handler.Entries[getKey(r)] = append(handler.Entries[getKey(r)], newEntry)
		lock.Unlock()

		return newEntry.Response.Code, nil
	}

	return 500, nil
}
