package cache

import (
	"time"
	"net/http"
	"net/http/httptest"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/pquerna/cachecontrol/cacheobject"
	"github.com/nicolasazrak/caddy-cache/storage"
)



type CacheHandler struct {
	Client storage.Storage
	Next   httpserver.Handler
}


func respond(response * storage.CachedResponse, w http.ResponseWriter) {
	for k, vs := range response.HeaderMap {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.Write(response.Body)
	w.WriteHeader(response.Code)
}

func shouldUseCache(r *http.Request) bool {
	if r.Method != "GET" && r.Method != "HEAD" {
		// Only cache Get and head request
		return false
	}

	// Add more logic like get params, ?nocache=true
	return true
}

func getCacheableStatus(req *http.Request, res *httptest.ResponseRecorder) *cacheobject.ObjectResults {
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
	return &rv
}

func isCacheable(rv *cacheobject.ObjectResults) bool {
	return len(rv.OutReasons) == 0 && rv.OutExpirationTime.Sub(time.Now().UTC()) > 0
}

func getKey(r *http.Request) string {
	return "contentcache:" + r.Host + ":" + r.Method + ":" + r.URL.Path
}


func (h CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {

	if !shouldUseCache(r) {
		return h.Next.ServeHTTP(w, r)
	}

	cached, err := h.Client.Get(getKey(r))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if cached == nil {
		rec := httptest.NewRecorder()
		status, err := h.Next.ServeHTTP(rec, r)

		response := storage.CachedResponse {
			Body: rec.Body.Bytes(),
			HeaderMap: rec.HeaderMap,
			Code: rec.Code,
		}

		cacheableStatus := getCacheableStatus(r, rec)
		if isCacheable(cacheableStatus) {

			err = h.Client.Set(getKey(r), &response, cacheableStatus.OutExpirationTime)
			if err != nil {
				return http.StatusInternalServerError, err
			}
		}

		respond(&response, w)
		return status, err
	} else {
		respond(cached, w)
		return cached.Code, nil
	}
}
