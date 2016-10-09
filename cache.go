package cache

import (
	"fmt"
	"time"
	"net/http"
	"net/http/httptest"
	"gopkg.in/redis.v4"
	"gopkg.in/vmihailenco/msgpack.v2"
	"github.com/mholt/caddy/caddyhttp/httpserver"
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


func respond(cached * CachedResponse, w http.ResponseWriter, wasCached bool) {
	for k, vs := range cached.HeaderMap {
		for _, v := range vs {
			// make-fmt.Println("key=", k, "value=", v)
			w.Header().Add(k, v)
		}
	}
	if wasCached {
		w.Header().Add("cache-status", "HIT")
	} else {
		w.Header().Add("cache-status", "MISS")
	}
	w.Write(cached.Body)
	w.WriteHeader(cached.Code)
}


func (h CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	val, err := h.Client.Get(r.URL.Path).Result()
	if err != nil && err != redis.Nil {
		fmt.Println(err)
		return 500, err
	}

	if err == redis.Nil {
		rec := httptest.NewRecorder()
		status, err := h.Next.ServeHTTP(rec, r)

		cached := CachedResponse {
			Body: rec.Body.Bytes(),
			HeaderMap: rec.HeaderMap,
			Code: rec.Code,
		}

		serialized, err := msgpack.Marshal(cached)

		if err != nil {
			return 500, nil
		}

		err = h.Client.Set(r.URL.Path, serialized, time.Duration(5) * time.Minute).Err()
		if err != nil {
			return 500, err
		}

		respond(&cached, w, false)
		return status, err
	} else {
		cached := CachedResponse{}
		msgpack.Unmarshal([]byte(val), &cached)
		respond(&cached, w, true)
		return cached.Code, nil
	}
}
