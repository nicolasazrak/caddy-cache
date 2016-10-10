package cache

import (
	"fmt"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/mholt/caddy"
	"github.com/nicolasazrak/caddy-cache/storage"
)

func init() {
	httpserver.RegisterDevDirective("cache", "root")
	caddy.RegisterPlugin("cache", caddy.Plugin{
		ServerType: "http",
		Action:     Setup,
	})
}

func Setup(c *caddy.Controller) error {
	handler := CacheHandler{}
	handler.Client = getHandler(c)

	err := handler.Client.Setup()
	if err != nil {
		return err
	}

	httpserver.GetConfig(c).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		handler.Next = next
		return handler
	})

	c.OnStartup(func() error {
		fmt.Println("Cache initialized")
		return nil
	})

	return nil
}


func getHandler(c *caddy.Controller) (storage.Storage) {
	c.Next() // Skip cache directive

	if !c.NextArg() {
		return &storage.MemoryStorage{}
	}

	return &storage.RedisStorage{ URL: c.Val() }
}
