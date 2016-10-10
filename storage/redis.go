package storage

import (
	"gopkg.in/vmihailenco/msgpack.v2"
	"gopkg.in/redis.v4"
	"time"
	"errors"
	"net/url"
	"strconv"
	"fmt"
)

type RedisStorage struct {
	URL string
	client *redis.Client
}


func (s *RedisStorage) Get(key string) (*CachedResponse, error) {
	cached, err := s.client.Get(key).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	if cached == "" {
		return nil, nil
	}
	cache := CachedResponse{}
	msgpack.Unmarshal([]byte(cached), &cache)
	return &cache, nil
}

func (s *RedisStorage) Set(key string, cached * CachedResponse, expiration time.Duration) error {
	nextCache, err := msgpack.Marshal(cached)
	if err != nil {
		return err
	}
	err = s.client.Set(key, nextCache, expiration).Err()
	if err != nil {
		return err
	}
	return nil
}

func (s *RedisStorage) Setup() error {
	redisOptions, err := parseRedisURL(s.URL)
	if err != nil {
		return err
	}
	s.client = redis.NewClient(redisOptions)
	_, err = s.client.Ping().Result()
	if err != nil {
		return errors.New("Could not connect to redis server for cache")
	}
	fmt.Println("Connected to redis...")
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