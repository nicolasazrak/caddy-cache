package storage

import (
	"io"
)

// ResponseStorage represents a possible storage for the entry
type ResponseStorage interface {
	io.Writer
	io.Closer
	Clean() error
	Flush() error
	GetReader() (io.ReadCloser, error)
}
