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
	redisURL, err := getRedisURL(c)
	if err != nil {
		return err
	}

	c.OnStartup(func() error {
		fmt.Println("Cache initialized")
		return nil
	})

	handler := CacheHandler{}
	handler.Client = &storage.RedisStorage {
		URL: redisURL,
	}

	err = handler.Client.Setup()

	if err != nil {
		return err
	}

	httpserver.GetConfig(c).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		handler.Next = next
		return handler
	})

	return nil
}


func getRedisURL(c *caddy.Controller) (string, error) {
	c.Next() // Skip cache directive

	if !c.NextArg() {       // expect at least one value
		return "", c.ArgErr()   // otherwise it's an error
	}

	return c.Val(), nil
}
