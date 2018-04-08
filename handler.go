package cache

import (
	"context"
	"net/http"
	"strings"

	"github.com/mholt/caddy"
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

var (
	contextKeysToPreserve = []caddy.CtxKey{
		httpserver.OriginalURLCtxKey,
		httpserver.ReplacerCtxKey,
		httpserver.RemoteUserCtxKey,
		httpserver.MitmCtxKey,
		httpserver.RequestIDCtxKey,
		"path_prefix",
		"mitm",
	}
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
func NewHandler(Next httpserver.Handler, config *Config) *Handler {
	return &Handler{
		Config:   config,
		Cache:    NewHTTPCache(),
		URLLocks: NewURLLock(),
		Next:     Next,
	}
}

/* Responses */

func copyHeaders(from http.Header, to http.Header) {
	for k, values := range from {
		for _, v := range values {
			to.Add(k, v)
		}
	}
}

func (handler *Handler) addStatusHeaderIfConfigured(w http.ResponseWriter, status string) {
	if rec, ok := w.(*httpserver.ResponseRecorder); ok {
		rec.Replacer.Set("cache_status", status)
	}

	if handler.Config.StatusHeader != "" {
		w.Header().Add(handler.Config.StatusHeader, status)
	}
}

func (handler *Handler) respond(w http.ResponseWriter, entry *HTTPCacheEntry, cacheStatus string) (int, error) {
	handler.addStatusHeaderIfConfigured(w, cacheStatus)

	copyHeaders(entry.Response.snapHeader, w.Header())
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

	// Range requests still not supported
	// It may happen that the previous request for this url has a successful response
	// but for another Range. So a special handling is needed
	if req.Header.Get("range") != "" {
		return false
	}

	if strings.ToLower(req.Header.Get("Connection")) == "upgrade" && strings.ToLower(req.Header.Get("Upgrade")) == "websocket" {
		return false
	}

	return true
}

func popOrNil(errChan chan error) (err error) {
	select {
	case err = <-errChan:
	default:
	}
	return
}

func (handler *Handler) fetchUpstream(req *http.Request) (*HTTPCacheEntry, error) {
	// Create a new empty response
	response := NewResponse()

	errChan := make(chan error, 1)

	// Do the upstream fetching in background
	go func(req *http.Request, response *Response) {
		// Create a new context to avoid terminating the Next.ServeHTTP when the original
		// request is closed. Otherwise if the original request is cancelled the other requests
		// will see a bad response that has the same contents the first request has
		updatedContext := context.Background()

		// The problem of cloning the context is that the original one has some values used by
		// other middlewares. If those values are not present they break, #22 is an example.
		// However there isn't a way to know which values a context has. I took the ones that
		// I found on caddy code. If in a future there are new ones this might break.
		// In that case this will have to change to another way
		for _, key := range contextKeysToPreserve {
			value := req.Context().Value(key)
			if value != nil {
				updatedContext = context.WithValue(updatedContext, key, value)
			}
		}

		updatedReq := req.WithContext(updatedContext)

		statusCode, upstreamError := handler.Next.ServeHTTP(response, updatedReq)
		errChan <- upstreamError

		// If status code was not set, this will not replace it
		// It will only ensure status code IS send
		response.WriteHeader(statusCode)

		// Wait the response body to be set.
		// If it is private it will be the original http.ResponseWriter
		// It is required to wait the body to prevent closing the response
		// before the body was set. If that happens the body will
		// stay locked waiting the response to be closed
		response.WaitBody()
		response.Close()
	}(req, response)

	// Wait headers to be sent
	response.WaitHeaders()

	// Create a new CacheEntry
	return NewHTTPCacheEntry(getKey(req), req, response, handler.Config), popOrNil(errChan)
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
			err := entry.setStorage(handler.Config)
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
		err := entry.setStorage(handler.Config)
		if err != nil {
			lock.Unlock()
			return 500, err
		}
	}

	handler.Cache.Put(r, entry)
	lock.Unlock()
	return handler.respond(w, entry, cacheMiss)
}
