package cache

import (
	"fmt"
	"gopkg.in/redis.v4"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/mholt/caddy"
	"errors"
	"net/url"
	"strconv"
)

func init() {
	httpserver.RegisterDevDirective("cache", "root")
	caddy.RegisterPlugin("cache", caddy.Plugin{
		ServerType: "http",
		Action:     Setup,
	})
}

func Setup(c *caddy.Controller) error {
	redisOptions, err := parseOptions(c)
	if err != nil {
		return err
	}

	c.OnStartup(func() error {
		fmt.Println("Cache initialized")
		return nil
	})

	handler := CacheHandler{}
	handler.Client = redis.NewClient(redisOptions)

	_, err = handler.Client.Ping().Result()
	if err != nil {
		return errors.New("Could not connect to redis server for cache")
	}

	httpserver.GetConfig(c).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		handler.Next = next
		return handler
	})

	return nil
}

func parseRedisURL(redisURL string) (*redis.Options, error) {
	parsed, err := url.Parse(redisURL)

	if err != nil {
		return nil, errors.New("Invalid redis url for cache")
	}

	if parsed.Scheme != "redis" {
		return nil, errors.New("Invalid " + parsed.Scheme + " protocol, it should be redis://")
	}

	db := 0
	if parsed.Path != "" {
		dbFromURl, err := strconv.Atoi(parsed.Path);
		if err == nil {
			return nil, errors.New("Invalid redis database, it must be a valid number")
		}
		db = dbFromURl
	}

	password := ""
	if parsed.User != nil {
		pwFromUrl, isSet := parsed.User.Password()
		if isSet {
			password = pwFromUrl
		}
	}

	return &redis.Options{
		Addr:     parsed.Host,
		Password: password,
		DB:       db,
	}, nil
}


func parseOptions(c *caddy.Controller) (*redis.Options, error) {
	c.Next() // Skip cache directive

	if !c.NextArg() {       // expect at least one value
		return nil, c.ArgErr()   // otherwise it's an error
	}

	redisURL := c.Val()

	return parseRedisURL(redisURL)
}
