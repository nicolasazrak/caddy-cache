package storage

import (
	"time"
	"net/http"
	"gopkg.in/vmihailenco/msgpack.v2"
)

type Storage interface {
	Get(key string) (*CachedResponse, error)
	Set(key string, cached * CachedResponse, expiration time.Time) error
	Setup() error
}

type CachedResponse struct {
	Code      int           // the HTTP response code from WriteHeader
	Body      []byte
	HeaderMap http.Header   // the HTTP response headers
}


// TODO, this needs to be improved for performance
func marshal(value *CachedResponse) ([]byte, error) {
	return msgpack.Marshal(value)
}

func unmarshall(value []byte) (*CachedResponse, error) {
	cache := CachedResponse{}
	msgpack.Unmarshal(value, &cache)
	return &cache, nil
}
