package cache

import (
	"time"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
)

const DEFAULT_MAX_AGE = time.Duration(60) * time.Second

type Config struct {
}

func init() {
	httpserver.RegisterDevDirective("cache", "root")
	caddy.RegisterPlugin("cache", caddy.Plugin{
		ServerType: "http",
		Action:     Setup,
	})
}

func Setup(c *caddy.Controller) error {
	handler := CacheHandler{
		Entries: map[string]*HttpCacheEntry{},
	}

	httpserver.GetConfig(c).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		handler.Next = next
		return handler
	})

	return nil
}
