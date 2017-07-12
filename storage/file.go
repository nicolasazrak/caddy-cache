package storage

import (
	"io"
	"io/ioutil"
	"os"
)

// FileStorage saves the content into a file
type FileStorage struct {
	file         *os.File
	subscription *Subscription
}

// NewFileStorage creates a new temp file that will be used as a the storage of the cache entry
func NewFileStorage(path string) (ResponseStorage, error) {
	file, err := ioutil.TempFile(path, "caddy-cache-")
	if err != nil {
		return nil, err
	}
	return &FileStorage{
		file:         file,
		subscription: NewSubscription(),
	}, nil
}

func (f *FileStorage) Write(p []byte) (n int, err error) {
	defer f.subscription.NotifyAll(len(p))
	return f.file.Write(p)
}

// Flush syncs the underlying file
func (f *FileStorage) Flush() error {
	defer f.subscription.NotifyAll(0)
	return f.file.Sync()
}

// Clean removes the file
func (f *FileStorage) Clean() error {
	f.subscription.WaitAll() // Wait until every subscriber ends waiting every result
	return os.Remove(f.file.Name())
}

// Close the underlying file
func (f *FileStorage) Close() error {
	f.subscription.Close()
	return f.file.Close()
}

// GetReader returns a new file descriptor to the same file
func (f *FileStorage) GetReader() (io.ReadCloser, error) {
	newFile, err := os.Open(f.file.Name())
	if err != nil {
		return nil, err
	}
	return &FileReader{
		content:      newFile,
		subscription: f.subscription.NewSubscriber(),
		unsubscribe:  f.subscription.RemoveSubscriber,
	}, nil
}

/////////////////////////////////////////

// FileReader is the common code to read the storages until the subscription channel is closed
type FileReader struct {
	subscription <-chan int
	content      io.ReadCloser
	unsubscribe  func(<-chan int)
}

func (r *FileReader) Read(p []byte) (n int, err error) {
	for range r.subscription {
		n, err := r.content.Read(p)
		if err != io.EOF {
			return n, err
		}
	}

	return r.content.Read(p)
}

// Close closes the underlying storage
func (r *FileReader) Close() error {
	err := r.content.Close()
	r.unsubscribe(r.subscription)
	return err
}
