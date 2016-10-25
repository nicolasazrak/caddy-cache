package cache

import (
	"time"
	"strings"
	"net/http"
	"net/http/httptest"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/pquerna/cachecontrol/cacheobject"
	"github.com/nicolasazrak/caddy-cache/storage"
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

func shouldUseCache(r *http.Request, config *Config) bool {
	// TODO Add more logic like get params, ?nocache=true

	if r.Method != "GET" && r.Method != "HEAD" {
		// Only cache Get and head request
		return false
	}

	// Range responses still not supported
	if r.Header.Get("accept-ranges") != "" {
		return false
	}

	for _, path := range config.CacheablePaths {
		if strings.HasPrefix(r.URL.Path, path) {
			return true
		}
	}

	return false
}

func getCacheableStatus(req *http.Request, res *httptest.ResponseRecorder, config *Config) (bool, time.Time) {
	reqDir, _ := cacheobject.ParseRequestCacheControl(req.Header.Get("Cache-Control"))
	resDir, _ := cacheobject.ParseResponseCacheControl(res.Header().Get("Cache-Control"))
	expiresHeader, _ := http.ParseTime(res.Header().Get("Expires"))
	dateHeader, _ := http.ParseTime(res.Header().Get("Date"))
	lastModifiedHeader, _ := http.ParseTime(res.Header().Get("Last-Modified"))

	obj := cacheobject.Object{
		RespDirectives:         resDir,
		RespHeaders:            res.Header(),
		RespStatusCode:         res.Code,
		RespExpiresHeader:      expiresHeader,
		RespDateHeader:         dateHeader,
		RespLastModifiedHeader: lastModifiedHeader,

		ReqDirectives: reqDir,
		ReqHeaders:    req.Header,
		ReqMethod:     req.Method,

		NowUTC: time.Now().UTC(),
	}

	rv := cacheobject.ObjectResults{}
	cacheobject.CachableObject(&obj, &rv)
	cacheobject.ExpirationObject(&obj, &rv)

	isCacheable := len(rv.OutReasons) == 0
	expiration := rv.OutExpirationTime
	if expiration.Before(time.Now().UTC().Add(time.Duration(1) * time.Second)) {
		// If expiration is before now use default MaxAge
		expiration = time.Now().UTC().Add(config.DefaultMaxAge)
	}

	return isCacheable, expiration
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

		isCacheable, expirationTime := getCacheableStatus(r, rec, h.Config)
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
