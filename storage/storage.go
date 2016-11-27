package storage

import (
	"time"
)

type Value interface{}

type Storage interface {
	Get(key string, matcher func(Value) bool) (Value, error)
	Push(key string, value Value, expiration time.Time) error
	Setup() error
}
