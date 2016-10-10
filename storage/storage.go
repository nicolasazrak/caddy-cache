package storage

import (
	"time"
	"net/http"
)

type Storage interface {
	Get(key string) (*CachedResponse, error)
	Set(key string, cached * CachedResponse, expiration time.Duration) error
	Setup() error
}

type CachedResponse struct {
	Code      int           // the HTTP response code from WriteHeader
	Body      []byte
	HeaderMap http.Header   // the HTTP response headers
}

