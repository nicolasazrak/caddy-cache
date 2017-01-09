package cache

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

type CachedRequest struct {
	HeaderMap http.Header // Headers are the only useful information
}

type CachedResponse struct {
	Code      int // the HTTP response code from WriteHeader
	Body      []byte
	HeaderMap http.Header // the HTTP response headers
}

type HttpCacheEntry struct {
	Request  *CachedRequest
	Response *CachedResponse
}

type CacheHandler struct {
	Config *Config
	Client *MemoryStorage
	Next   httpserver.Handler
}

func respond(response *CachedResponse, w http.ResponseWriter) {
	for k, values := range response.HeaderMap {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(response.Code)
	w.Write(response.Body)
}

func getKey(r *http.Request) string {
	key := r.Method + " " + r.Host + r.URL.Path

	q := r.URL.Query().Encode()
	if len(q) > 0 {
		key += "?" + q
	}

	return key
}

func (h *CacheHandler) chooseIfVary(r *http.Request) func(Value) bool {
	return func(value Value) bool {
		entry := value.(*HttpCacheEntry)
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

func (h CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	if !shouldUseCache(r) {
		h.AddStatusHeaderIfConfigured(w, "skip")
		return h.Next.ServeHTTP(w, r)
	}

	value, err := h.Client.Get(getKey(r), h.chooseIfVary(r))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if value == nil {
		rec := NewStreamedRecorder(w)

		h.AddStatusHeaderIfConfigured(w, "miss")

		_, err := h.Next.ServeHTTP(rec, r)

		response := &HttpCacheEntry{
			Request: &CachedRequest{
				HeaderMap: r.Header,
			},
			Response: &CachedResponse{
				Body:      rec.Body.Bytes(),
				HeaderMap: h.RemoveStatusHeaderIfConfigured(rec.Result().Header),
				Code:      rec.Result().StatusCode,
			},
		}

		isCacheable, expirationTime, err := getCacheableStatus(r, rec, h.Config)

		if err != nil {
			return 500, err
		}

		if isCacheable {
			err = h.Client.Push(getKey(r), response, expirationTime)
			if err != nil {
				return http.StatusInternalServerError, err
			}
		}

		return response.Response.Code, err
	} else {
		cached := value.(*HttpCacheEntry)
		h.AddStatusHeaderIfConfigured(w, "hit")
		respond(cached.Response, w)
		return cached.Response.Code, nil
	}
}
