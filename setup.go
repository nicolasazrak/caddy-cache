package cache

import (
	"net/http"
	"strings"
	"time"

	"os"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
)

var (
	defaultStatusHeader = "X-Cache-Status"
	defaultLockTimeout  = time.Duration(5) * time.Minute
	defaultMaxAge       = time.Duration(5) * time.Minute
	defaultPath         = ""
)

type Config struct {
	StatusHeader      string
	UpstreamHeaders   http.Header
	DownstreamHeaders http.Header
	DefaultMaxAge     time.Duration
	LockTimeout       time.Duration
	CacheRules        []CacheRule
	Path              string
	CacheKeyTemplate  string
}

func init() {
	// httpserver.RegisterDevDirective("cache", "root")
	caddy.RegisterPlugin("cache", caddy.Plugin{
		ServerType: "http",
		Action:     Setup,
	})
}

// Setup creates a new cache middleware
func Setup(c *caddy.Controller) error {
	config, err := cacheParse(c)

	if err != nil {
		return err
	}

	httpserver.GetConfig(c).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		return NewHandler(next, config)
	})

	c.OnStartup(func() error {
		if config.Path == "" {
			return nil
		}
		return os.MkdirAll(config.Path, 0600)
	})

	return nil
}

// defaultCacheKeyTemplate is the placeholder template that will be used to
// generate the cache key.
const defaultCacheKeyTemplate = "{method} {host}{path}?{query}"

func emptyConfig() *Config {
	return &Config{
		StatusHeader:      defaultStatusHeader,
		UpstreamHeaders:   make(http.Header),
		DownstreamHeaders: make(http.Header),
		DefaultMaxAge:     defaultMaxAge,
		LockTimeout:       defaultLockTimeout,
		CacheRules:        []CacheRule{},
		Path:              defaultPath,
		CacheKeyTemplate:  defaultCacheKeyTemplate,
	}
}

func cacheParse(c *caddy.Controller) (*Config, error) {
	config := emptyConfig()

	c.Next() // Skip "cache" literal

	if len(c.RemainingArgs()) > 1 {
		return nil, c.Err("Unexpected value " + c.Val())
	}

	for c.NextBlock() {
		parameter := c.Val()
		args := c.RemainingArgs()

		switch parameter {
		case "status_header":
			if len(args) != 1 {
				return nil, c.Err("Invalid usage of status_header in cache config.")
			}
			config.StatusHeader = args[0]
		case "header_upstream":
			if len(args) != 2 {
				// When removing a header, the value can be optional.
				if !strings.HasPrefix(args[0], "-") {
					return nil, c.ArgErr()
				}
			}
			config.UpstreamHeaders.Add(args[0], args[1])
		case "header_downstream":
			if len(args) != 2 {
				// When removing a header, the value can be optional.
				if !strings.HasPrefix(args[0], "-") {
					return nil, c.ArgErr()
				}
			}
			config.DownstreamHeaders.Add(args[0], args[1])
		case "lock_timeout":
			if len(args) != 1 {
				return nil, c.Err("Invalid usage of lock_timeout in cache config.")
			}
			duration, err := time.ParseDuration(c.Val())
			if err != nil {
				return nil, c.Err("lock_timeout: Invalid duration " + c.Val())
			}
			config.LockTimeout = duration
		case "default_max_age":
			if len(args) != 1 {
				return nil, c.Err("Invalid usage of default_max_age in cache config.")
			}
			duration, err := time.ParseDuration(c.Val())
			if err != nil {
				return nil, c.Err("default_max_age: Invalid duration " + c.Val())
			}
			config.DefaultMaxAge = duration
		case "path":
			if len(args) != 1 {
				return nil, c.Err("Invalid usage of path in cache config.")
			}
			config.Path = args[0]
		case "match_header":
			if len(args) < 2 {
				return nil, c.Err("Invalid usage of match_header in cache config.")
			}
			cacheRule := &HeaderCacheRule{Header: args[0], Value: args[1:]}
			config.CacheRules = append(config.CacheRules, cacheRule)
		case "match_path":
			if len(args) != 1 {
				return nil, c.Err("Invalid usage of match_path in cache config.")
			}
			cacheRule := &PathCacheRule{Path: args[0]}
			config.CacheRules = append(config.CacheRules, cacheRule)
		case "cache_key":
			if len(args) != 1 {
				return nil, c.Err("Invalid usage of cache_key in cache config.")
			}
			config.CacheKeyTemplate = args[0]
		default:
			return nil, c.Err("Unknown cache parameter: " + parameter)
		}
	}

	return config, nil
}
