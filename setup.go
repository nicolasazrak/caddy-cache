package cache

import (
	"fmt"
	"time"
	"strconv"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/nicolasazrak/caddy-cache/storage"
)

const DEFAULTMAXAGE = 60

func init() {
	httpserver.RegisterDevDirective("cache", "root")
	caddy.RegisterPlugin("cache", caddy.Plugin{
		ServerType: "http",
		Action:     Setup,
	})
}

type Config struct {
	CacheablePaths []string
	RedisURL       string
	DefaultMaxAge  time.Duration
}

func Setup(c *caddy.Controller) error {
	config, err := cacheParse(c)

	if err != nil {
		return err
	}

	handler := CacheHandler{
		Config: config,
		Client: getHandler(config),
	}

	httpserver.GetConfig(c).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		handler.Next = next
		return handler
	})

	c.OnStartup(func() error {
		err = handler.Client.Setup()
		if err != nil {
			return err
		}
		fmt.Println("Cache initialized")
		return nil
	})

	return nil
}

func cacheParse(c *caddy.Controller) (*Config, error) {
	config := Config{
		CacheablePaths: []string{},
		DefaultMaxAge: time.Duration(DEFAULTMAXAGE) * time.Second,
		RedisURL: "",
	}

	c.Next()
	if !c.NextArg() {
		return nil, c.Err("Missing cache path")
	}

	config.CacheablePaths = append(config.CacheablePaths, c.Val())

	for c.NextBlock() {
			parameter := c.Val()
			switch parameter {

			case "path":
				args := c.RemainingArgs()
				if len(args) != 1 {
					return nil, c.Err("Invalid usage of path in cache config.")
				} else{
					config.CacheablePaths = append(config.CacheablePaths, args[0])
				}
			case "default_max_age" :
				args := c.RemainingArgs()
				if len(args) != 1 {
					return nil, c.Err("Invalid usage of default_max_age in cache config.")
				} else{
					val, err := strconv.Atoi(args[0])
					if err != nil || val < 0 {
						return nil, c.Err("Invalid value of default_max_age")
					}
					config.DefaultMaxAge = time.Duration(val) * time.Second
				}
			case "redis":
				args := c.RemainingArgs()
				if len(args) != 1 {
					return nil, c.Err("Invalid usage of redis in cache config")
				} else{
					config.RedisURL = args[0]
				}
			default:
				return nil, c.Err("Unknown cache parameter: " + parameter)
			}
	}

	return &config, nil
}

func getHandler(config *Config) storage.Storage {
	if config.RedisURL == "" {
		return &storage.MemoryStorage{}
	}

	return &storage.RedisStorage{URL: config.RedisURL}
}
