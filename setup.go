package cache

import (
	"fmt"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/nicolasazrak/caddy-cache/storage"
	"strconv"
	"time"
)

const DEFAULT_MAX_AGE = time.Duration(60) * time.Second

type Config struct {
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
		Client: &storage.MemoryStorage{},
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
		CacheRules:    []CacheRule{},
		DefaultMaxAge: DEFAULT_MAX_AGE,
		StatusHeader:  "",
	}

	c.Next() // Skip "cache" literal

	if len(c.RemainingArgs()) > 1 {
		return nil, c.Err("Unexpected value " + c.Val())
	}

	for c.NextBlock() {
		parameter := c.Val()
		switch parameter {

		case "match":
			args := c.RemainingArgs()
			if len(args) != 0 {
				return nil, c.Err("Invalid usage of match in cache config.")
			} else {
				cacheRules, err := parseMatchRules(c)
				if err != nil {
					return nil, err
				}
				config.CacheRules = cacheRules
			}
		case "default_max_age":
			args := c.RemainingArgs()
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
			args := c.RemainingArgs()
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

func parseMatchRules(c *caddy.Controller) ([]CacheRule, error) {
	if c.Next() && c.Val() != "{" { // Hack to make work nested Blocks
		return nil, c.Err("Invalid syntax on match directive in cache configuration")
	}

	rules := []CacheRule{}

	for c.NextBlock() {
		condition := c.Val()
		switch condition {
		case "header":
			args := c.RemainingArgs()
			if len(args) < 2 {
				return nil, c.Err("Invalid number of arguments in header condition of match in cache config.")
			} else {
				rules = append(rules, &HeaderCacheRule{
					Header: args[0],
					Value:  args[1:],
				})
			}
		case "path":
			args := c.RemainingArgs()
			if len(args) != 1 {
				return nil, c.Err("Invalid number of arguments in path condition of match in cache config.")
			} else {
				rules = append(rules, &PathCacheRule{
					Path: args[0],
				})
			}
		default:
			return nil, c.Err(fmt.Sprintf("Unknown condition %s on match parameter of cache directive", condition))
		}
	}

	c.IncrNest() // Hack to make work nested Blocks
	return rules, nil
}
