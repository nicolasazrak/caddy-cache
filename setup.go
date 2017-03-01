package cache

import (
	"fmt"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"path"
	"runtime"
	"strconv"
	"time"
)

const DEFAULT_MAX_AGE = time.Duration(60) * time.Second

type Config struct {
	Storage       Storage
	CacheRules    []CacheRule
	DefaultMaxAge time.Duration
	StatusHeader  string
}

func init() {
	httpserver.RegisterDevDirective("cache", "root")
	caddy.RegisterPlugin("cache", caddy.Plugin{
		ServerType: "http",
		Action:     Setup,
	})
}

func Setup(c *caddy.Controller) error {
	config, err := cacheParse(c)

	if err != nil {
		return err
	}

	handler := CacheHandler{
		Config: config,
		Cache:  NewCache(config.Storage),
	}

	httpserver.GetConfig(c).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		handler.Next = next
		return handler
	})

	c.OnStartup(func() error {
		err = handler.Cache.Setup()
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
		Storage:       NewMMapStorage(path.Join("/", "tmp", "caddy-cache")),
		CacheRules:    []CacheRule{},
		DefaultMaxAge: DEFAULT_MAX_AGE,
		StatusHeader:  "",
	}

	if runtime.GOOS == "windows" {
		config.Storage = NewMemoryStorage()
	}

	c.Next() // Skip "cache" literal

	if len(c.RemainingArgs()) > 1 {
		return nil, c.Err("Unexpected value " + c.Val())
	}

	for c.NextBlock() {
		parameter := c.Val()
		args := c.RemainingArgs()

		switch parameter {

		case "match":
			if len(args) == 0 {
				return nil, c.Err("Invalid usage of match in cache config.")
			} else {
				cacheRule, err := parseMatchRules(c, args)
				if err != nil {
					return nil, err
				}
				config.CacheRules = append(config.CacheRules, cacheRule)
			}
		case "storage":
			if len(args) == 0 {
				return nil, c.Err("Invalid storage directive, specify: memory or mmap")
			}
			switch args[0] {
			case "mmap":
				if runtime.GOOS == "windows" {
					return nil, c.Err("MMap storage is not available in Windows")
				}
				if len(args) != 2 {
					return nil, c.Err("Invalid mmap configs")
				}
				config.Storage = NewMMapStorage(args[1])
			case "memory":
				config.Storage = NewMemoryStorage()
			default:
				return nil, c.Err("Unknown storage engine " + args[0])
			}
		case "default_max_age":
			if len(args) != 1 {
				return nil, c.Err("Invalid usage of default_max_age in cache config.")
			} else {
				val, err := strconv.Atoi(args[0])
				if err != nil || val < 0 {
					return nil, c.Err("Invalid value of default_max_age")
				}
				config.DefaultMaxAge = time.Duration(val) * time.Second
			}
		case "status_header":
			if len(args) != 1 {
				return nil, c.Err("Invalid usage of status_header in cache config.")
			} else {
				config.StatusHeader = args[0]
			}
		default:
			return nil, c.Err("Unknown cache parameter: " + parameter)
		}
	}

	return &config, nil
}

func parseMatchRules(c *caddy.Controller, args []string) (CacheRule, error) {
	switch args[0] {
	case "header":
		if len(args) < 3 {
			return nil, c.Err("Invalid number of arguments in header condition of match in cache config.")
		} else {
			return &HeaderCacheRule{
				Header: args[1],
				Value:  args[2:],
			}, nil
		}
	case "path":
		if len(args) != 2 {
			return nil, c.Err("Invalid number of arguments in path condition of match in cache config.")
		} else {
			return &PathCacheRule{
				Path: args[1],
			}, nil
		}
	default:
		return nil, c.Err(fmt.Sprintf("Unknown condition %s on match parameter of cache directive", args[0]))
	}
}
