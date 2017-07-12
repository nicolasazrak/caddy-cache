package storage

import (
	"errors"
	"io"
	"net/http"
)

// NoStorage writes the content directly into the ResponseWriter
// TODO remove this
type NoStorage struct {
	w http.ResponseWriter
}

// WrapResponseWriter wraps an http.ResponseWriter and gives it
// the ResponseStorage interface
func WrapResponseWriter(w http.ResponseWriter) ResponseStorage {
	return &NoStorage{
		w: w,
	}
}

func (b *NoStorage) Write(p []byte) (n int, err error) {
	return b.w.Write(p)
}

// Flush does nothing in a buffer
func (b *NoStorage) Flush() error {
	if f, ok := b.w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

// Clean does nothing in the buffer it will be garbage collected eventually
func (b *NoStorage) Clean() error {
	return nil
}

// Close the storage entry
func (b *NoStorage) Close() error {
	return nil
}

// GetReader returns the same buffer
func (b *NoStorage) GetReader() (io.ReadCloser, error) {
	return nil, errors.New("Private responses are no readable")
}
