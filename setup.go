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

// Setup creates a new cache middleware
func Setup(c *caddy.Controller) error {
	handler := &Handler{
		Entries:  map[string][]*HTTPCacheEntry{},
		URLLocks: NewURLLock(),
	}

	httpserver.GetConfig(c).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		handler.Next = next
		return handler
	})

	return nil
}
