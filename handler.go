package cache

import (
	"fmt"
	"io"
	"net/http"

	"os"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

type CacheHandler struct {
	Config *Config
	// Cache  *Cache
	Entries map[string]*HttpCacheEntry
	Next    httpserver.Handler
}

type Response struct {
	Code         int // the HTTP response code from WriteHeader
	BodyFilePath string
	HeaderMap    http.Header // the HTTP response headers
}

type HttpCacheEntry struct {
	// isPublic   bool
	// Expiration time.Time
	closed      bool
	subscribers []chan struct{}
	Request     *http.Request
	Response    *Response
}

func (e *HttpCacheEntry) subscribe() <-chan struct{} {
	newSubscriber := make(chan struct{})
	if e.closed {
		fmt.Println("There was a subscribe but the entry is already closed")
		close(newSubscriber)
		return newSubscriber
	}
	fmt.Println("Adding subscriber to an opened entry")
	e.subscribers = append(e.subscribers, newSubscriber)
	return newSubscriber
}

func (e *HttpCacheEntry) notifySubscribers() {
	// fmt.Println("Notifying subscribers of more content", len(e.subscribers))
	for _, subscriber := range e.subscribers {
		subscriber <- struct{}{}
	}
}

func (e *HttpCacheEntry) close() {
	e.closed = true
	fmt.Println("Closing the entry", len(e.subscribers))
	for _, subscriber := range e.subscribers {
		close(subscriber)
	}
}

// GetEntry returns an http entry to serve
// If the response is in cache it will return the previous entry
// In case is not there it will request it to upstream
func (handler CacheHandler) GetEntry(r *http.Request) (*HttpCacheEntry, error) {
	fmt.Println("GetEntry", handler.Entries, r.URL.Path)
	previousEntry, exists := handler.Entries[r.URL.Path]
	if exists {
		fmt.Println("cached")
		return previousEntry, nil
	}

	fmt.Println("not cached")
	upstreamHandler := NewUpstreamHandler(handler.Next, r)
	newEntryResultChan := upstreamHandler.FetchUpstream()
	fmt.Println("Waiting newEntryResultChan...")
	newResult := <-newEntryResultChan
	if newResult.err != nil {
		fmt.Println("There was an error")
		return nil, newResult.err
	}

	fmt.Println("Saving entry in cache")
	handler.Entries[r.URL.Path] = newResult.entry
	return newResult.entry, nil
}

func (handler CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	fmt.Println("Handling request........")
	entry, err := handler.GetEntry(r)

	if err != nil {
		return 500, err
	}

	f, err := os.Open(entry.Response.BodyFilePath)
	if err != nil {
		return 500, err
	}
	defer f.Close()

	for k, values := range entry.Response.HeaderMap {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(entry.Response.Code)
	moreContentChan := entry.subscribe()

	io.Copy(w, f)
	for range moreContentChan {
		io.Copy(w, f)
	}
	io.Copy(w, f)

	return entry.Response.Code, nil
}
