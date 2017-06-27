package cache

import (
	"bytes"
	"sync"
)

// ConcurrentBuffer is a thread-safe buffer implementation
type ConcurrentBuffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *ConcurrentBuffer) Read(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Read(p)
}

func (b *ConcurrentBuffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}

func (b *ConcurrentBuffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}
