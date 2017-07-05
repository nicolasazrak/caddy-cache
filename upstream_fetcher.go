package cache

import (
	"net/http"
	"time"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

type StorageBuilder struct {
	request *http.Request
	config  *Config
}

func (sb *StorageBuilder) BuildStorage(response *Response) (bool, time.Time, ResponseStorage, error) {
	isPublic, expiration, err := getCacheableStatus(sb.request, response, sb.config)

	if !isPublic || err != nil {
		storage, err := NewBufferEntryStorage()
		return false, time.Now(), storage, err
	}

	storage, err := NewFileEntryStorage()
	return isPublic, expiration, storage, err
}

// UpstreamResult represent the result of fetching upstream
type UpstreamResult struct {
	err   error
	entry *HTTPCacheEntry
}

// FetchUpstream fetchs upstream and returns a channel that will emit an UpstreamResult
func FetchUpstream(Next httpserver.Handler, req *http.Request, config *Config) UpstreamResult {
	response := NewResponse(&StorageBuilder{req, config})
	entry := NewHTTPCacheEntry(req, response)
	endChan := make(chan UpstreamResult)

	go func(entry *HTTPCacheEntry, endChan chan UpstreamResult) {
		statusCode, err := Next.ServeHTTP(entry.Response, entry.Request)
		entry.Response.replaceCode(statusCode)
		entry.Close()
		endChan <- UpstreamResult{entry: entry, err: err}
	}(entry, endChan)

	select {
	case <-response.headersChan:
		return UpstreamResult{
			entry: entry,
		}
	case err := <-response.errorChan:
		response.replaceCode(500)
		return UpstreamResult{
			entry: entry,
			err:   err,
		}
	case result := <-endChan:
		return result
	}
}
