package cache

import (
	"net/http"

	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/mholt/caddy"
	"net/http/httptest"
	"fmt"
	"gopkg.in/redis.v4"
	"gopkg.in/vmihailenco/msgpack.v2"
	"time"
)

type Rule struct {
	Path    string
}

type CacheHandler struct {
	Rules []Rule
	Next  httpserver.Handler
}

func init() {
	fmt.Println("Init!")
	httpserver.RegisterDevDirective("cache", "root")
	caddy.RegisterPlugin("cache", caddy.Plugin{
		ServerType: "http",
		Action:     Setup,
	})
}

var client *redis.Client

func Setup(c *caddy.Controller) error {
	rules, err := parse(c)
	if err != nil {
		return err
	}

	c.OnStartup(func() error {
		return nil
	})

	client = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})


	cfg := httpserver.GetConfig(c)
	mid := func(next httpserver.Handler) httpserver.Handler {
		return CacheHandler{
			Rules: rules,
			Next: next,
		}
	}
	cfg.AddMiddleware(mid)

	return nil
}


func parse(c *caddy.Controller) ([]Rule, error) {
	return []Rule{}, nil
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
	val, err := client.Get(r.URL.Path).Result()
	if err != nil && err != redis.Nil {
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

		err = client.Set(r.URL.Path, serialized, time.Duration(5) * time.Minute).Err()
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
