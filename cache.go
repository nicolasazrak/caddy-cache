package cache

import (
	"time"
	"net/http"
	"net/http/httptest"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/nicolasazrak/caddy-cache/storage"
	"github.com/pquerna/cachecontrol"
)



type CacheHandler struct {
	Config *Config
	Client storage.Storage
	Next   httpserver.Handler
}


func respond(response * storage.CachedResponse, w http.ResponseWriter) {
	for k, values := range response.HeaderMap {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(response.Code)
	w.Write(response.Body)
}

func shouldUseCache(req *http.Request, config *Config) bool {
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
		return false, expiration, nil
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
	return "contentcache:" + r.Host + ":" + r.Method + ":" + r.URL.Path
}


func (h CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {

	if !shouldUseCache(r, h.Config) {
		return h.Next.ServeHTTP(w, r)
	}

	cached, err := h.Client.Get(getKey(r))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if cached == nil {
		rec := httptest.NewRecorder()
		_, err := h.Next.ServeHTTP(rec, r)

		response := storage.CachedResponse {
			Body: rec.Body.Bytes(),
			HeaderMap: rec.HeaderMap,
			Code: rec.Code,
		}

		isCacheable, expirationTime, err := getCacheableStatus(r, rec, h.Config)

		if err != nil {
			return 500, err
		}

		if isCacheable {
			err = h.Client.Set(getKey(r), &response, expirationTime)
			if err != nil {
				return http.StatusInternalServerError, err
			}
		}

		respond(&response, w)
		return response.Code, err
	} else {
		respond(cached, w)
		return cached.Code, nil
	}
}
