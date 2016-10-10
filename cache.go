package cache

import (
	"fmt"
	"time"
	"net/http"
	"net/http/httptest"
	"gopkg.in/redis.v4"
	"gopkg.in/vmihailenco/msgpack.v2"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/pquerna/cachecontrol/cacheobject"
)



type CacheHandler struct {
	Client *redis.Client
	Next   httpserver.Handler
}


type CachedResponse struct {
	Code      int           // the HTTP response code from WriteHeader
	Body      []byte
	HeaderMap http.Header   // the HTTP response headers
}


func respond(response * CachedResponse, w http.ResponseWriter) {
	for k, vs := range response.HeaderMap {
		for _, v := range vs {
			// make-fmt.Println("key=", k, "value=", v)
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
	return len(rv.OutReasons) == 0 && getTTL(rv) > 0
}

func getTTL(rv *cacheobject.ObjectResults) time.Duration {
	return rv.OutExpirationTime.Sub(time.Now().UTC())
}

func getKey(r *http.Request) string {
	return "contentcache:" + r.Host + ":" + r.Method + ":" + r.URL.Path
}

func (h CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {

	if !shouldUseCache(r) {
		return h.Next.ServeHTTP(w, r)
	}

	cached, err := h.Client.Get(getKey(r)).Result()
	if err != nil && err != redis.Nil {
		fmt.Println(err)
		return 500, err
	}

	if err == redis.Nil {
		rec := httptest.NewRecorder()
		status, err := h.Next.ServeHTTP(rec, r)

		response := CachedResponse {
			Body: rec.Body.Bytes(),
			HeaderMap: rec.HeaderMap,
			Code: rec.Code,
		}

		cacheableStatus := getCacheableStatus(r, rec)
		if isCacheable(cacheableStatus) {
			nextCache, err := msgpack.Marshal(response)

			if err != nil {
				return 500, nil
			}

			err = h.Client.Set(getKey(r), nextCache, getTTL(cacheableStatus)).Err()
			if err != nil {
				return 500, err
			}
		}

		respond(&response, w)
		return status, err
	} else {
		cache := CachedResponse{}
		msgpack.Unmarshal([]byte(cached), &cache)
		respond(&cache, w)
		return cache.Code, nil
	}
}
