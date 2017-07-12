// +build linux

package storage

import (
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"syscall"
)

// MmapStorage saves the content into a file
// using mmap to avoid syscalls when readers need it
type MmapStorage struct {
	file         *os.File
	data         []byte
	size         int
	offset       int
	subscription *Subscription
}

// NewMmapStorage creates a new temp file that will be used as a the storage of the cache entry
func NewMmapStorage(path string, size int) (ResponseStorage, error) {
	file, err := ioutil.TempFile(path, "caddy-cache-")
	if err != nil {
		return nil, err
	}

	err = syscall.Fallocate(int(file.Fd()), 0, 0, int64(size))
	if err != nil {
		file.Close()
		return nil, err
	}

	flags := syscall.PROT_READ | syscall.PROT_WRITE
	data, err := syscall.Mmap(int(file.Fd()), 0, size, flags, syscall.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, err
	}

	err = syscall.Madvise(data, syscall.MADV_SEQUENTIAL|syscall.MADV_WILLNEED)
	if err != nil {
		syscall.Munmap(data)
		file.Close()
		return nil, err
	}

	s := &MmapStorage{
		data:         data,
		file:         file,
		size:         size,
		subscription: NewSubscription(),
	}

	runtime.SetFinalizer(s, (*MmapStorage).Close) // Avoids leaking file descriptors
	return s, nil
}

func (f *MmapStorage) Write(p []byte) (n int, err error) {
	n = copy(f.data[f.offset:], p)
	f.offset += n
	f.subscription.NotifyAll(n)
	return n, nil
}

func (f *MmapStorage) Flush() error {
	f.subscription.NotifyAll(0)
	return nil
}

// Clean removes the file
func (f *MmapStorage) Clean() error {
	f.subscription.WaitAll() // Wait until every subscriber ends waiting every result

	err := syscall.Munmap(f.data)
	if err != nil {
		return err
	}

	err = f.file.Close()
	if err != nil {
		return err
	}
	return os.Remove(f.file.Name())
}

// Close the underlying file
func (f *MmapStorage) Close() error {
	runtime.SetFinalizer(f, nil)

	f.subscription.Close()
	return nil
}

// GetReader returns a new file descriptor to the same file
func (f *MmapStorage) GetReader() (io.ReadCloser, error) {
	return &MmapReader{
		data:         f.data,
		size:         f.offset,
		subscription: f.subscription.NewSubscriber(),
		unsubscribe:  f.subscription.RemoveSubscriber,
	}, nil
}

////////////////////////////////////////////////

type MmapReader struct {
	data         []byte
	offset       int
	size         int
	subscription <-chan int
	unsubscribe  func(<-chan int)
}

func (m *MmapReader) Read(p []byte) (int, error) {
	for newBytes := range m.subscription {
		m.size += newBytes
	}

	if m.offset < m.size {
		n := copy(p, m.data[m.offset:m.size])
		m.offset += n

		return n, nil
	}

	return 0, io.EOF
}

func (m *MmapReader) Close() error {
	m.unsubscribe(m.subscription)
	return nil
}
