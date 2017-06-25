package cache

import (
	"fmt"
	"io"
	"net/http"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

type CacheHandler struct {
	Config *Config
	// Cache  *Cache
	Entries map[string]*HttpCacheEntry
	Next    httpserver.Handler
}

// GetEntry returns an http entry to serve
// If the response is in cache it will return the previous entry
// In case is not there it will request it to upstream
func (handler CacheHandler) GetEntry(r *http.Request) (*HttpCacheEntry, error) {
	previousEntry, exists := handler.Entries[r.URL.Path]
	if exists {
		return previousEntry, nil
	}

	newResult := <-FetchUpstream(handler.Next, r)
	if newResult.err != nil {
		return nil, newResult.err
	}

	newEntry := newResult.entry
	if newEntry.isPublic {
		handler.Entries[r.URL.Path] = newEntry
	}
	return newEntry, nil
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

func (handler CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	if !shouldUseCache(r) {
		return handler.Next.ServeHTTP(w, r)
	}

	fmt.Println("Handling request........")
	entry, err := handler.GetEntry(r)

	if err != nil {
		return 500, err
	}

	body, err := entry.GetBodyReader()
	if err != nil {
		fmt.Println(err)
		return 500, err
	}
	defer body.Close()

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
