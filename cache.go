package cache

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"time"

	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/nicolasazrak/caddy-cache/storage"
	"github.com/pquerna/cachecontrol"
)

type CachedRequest struct {
	HeaderMap http.Header // Headers are the only useful information
}

type CachedResponse struct {
	Code      int // the HTTP response code from WriteHeader
	Body      []byte
	HeaderMap http.Header // the HTTP response headers
}

type CacheEntry struct {
	Request  *CachedRequest
	Response *CachedResponse
}

type CacheHandler struct {
	Config *Config
	Client storage.Storage
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

func getCacheableStatus(req *http.Request, res *httptest.ResponseRecorder, config *Config) (bool, time.Time, error) {
	reasonsNotToCache, expiration, err := cachecontrol.CachableResponse(req, res.Result(), cachecontrol.Options{})

	if err != nil {
		return false, time.Now(), err
	}

	canBeStored := len(reasonsNotToCache) == 0

	if !canBeStored {
		return false, time.Now(), nil
	}

	varyHeaders, ok := res.HeaderMap["Vary"]
	if ok && varyHeaders[0] == "*" {
		return false, time.Now(), nil
	}

	hasExplicitExpiration := expiration.After(time.Now().UTC())

	if expiration.Before(time.Now().UTC().Add(time.Duration(1) * time.Second)) {
		// If expiration is not specified or is before now use default MaxAge
		expiration = time.Now().UTC().Add(config.DefaultMaxAge)
	}

	anyCacheRulesMatches := false
	for _, rule := range config.CacheRules {
		if rule.matches(req, res) {
			anyCacheRulesMatches = true
			break
		}
	}

	if err != nil {
		return false, time.Now(), err
	}

	return anyCacheRulesMatches || hasExplicitExpiration, expiration, nil
}

func getKey(r *http.Request) string {
	key := r.Method + " " + r.Host + r.URL.Path

	q := r.URL.Query().Encode()
	if len(q) > 0 {
		key += "?" + q
	}

	return key
}

func (h CacheHandler) chooseIfVary(r *http.Request) func(storage.Value) bool {
	return func(value storage.Value) bool {
		entry := value.(*CacheEntry)
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

func (h CacheHandler) AddStatusHeader(w http.ResponseWriter, status string) {
	if h.Config.StatusHeader != "" {
		w.Header().Add(h.Config.StatusHeader, status)
	}
}

func (h CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	if !shouldUseCache(r) {
		code, err := h.Next.ServeHTTP(w, r)
		h.AddStatusHeader(w, "skip")
		return code, err
	}

	value, err := h.Client.Get(getKey(r), h.chooseIfVary(r))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if value == nil {
		rec := httptest.NewRecorder()
		_, err := h.Next.ServeHTTP(rec, r)

		response := &CacheEntry{
			Request: &CachedRequest{
				HeaderMap: r.Header,
			},
			Response: &CachedResponse{
				Body:      rec.Body.Bytes(),
				HeaderMap: rec.HeaderMap,
				Code:      rec.Code,
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

		h.AddStatusHeader(w, "miss")
		respond(response.Response, w)
		return response.Response.Code, err
	} else {
		cached := value.(*CacheEntry)
		respond(cached.Response, w)
		h.AddStatusHeader(w, "hit")
		return cached.Response.Code, nil
	}
}
