package cache

import (
	"errors"
	"net/http"
	"sync"

	"github.com/nicolasazrak/caddy-cache/storage"
)

type Response struct {
	Code       int         // the HTTP response code from WriteHeader
	HeaderMap  http.Header // the HTTP response headers
	body       storage.ResponseStorage
	snapHeader http.Header // copy of HTTP headeres at writeHeader time

	wroteHeader   bool
	firstByteSent bool

	bodyLock    *sync.RWMutex
	closedLock  *sync.RWMutex
	headersLock *sync.RWMutex
	closeNotify chan bool
}

// NewResponse returns an initialized Response.
func NewResponse() *Response {
	r := &Response{
		Code:        200,
		HeaderMap:   http.Header{},
		body:        nil,
		closeNotify: make(chan bool, 1),
		bodyLock:    new(sync.RWMutex),
		closedLock:  new(sync.RWMutex),
		headersLock: new(sync.RWMutex),
	}

	r.bodyLock.Lock()
	r.closedLock.Lock()
	r.headersLock.Lock()
	return r
}

// Header returns the response headers.
func (rw *Response) Header() http.Header {
	return rw.HeaderMap
}

// TODO check this
func (rw *Response) CloseNotify() <-chan bool {
	return rw.closeNotify
}

// writeHeader writes a header if it was not written yet and
// detects Content-Type if needed.
//
// bytes or str are the beginning of the response body.
// We pass both to avoid unnecessarily generate garbage
// in rw.WriteString which was created for performance reasons.
// Non-nil bytes win.
func (rw *Response) writeHeader(b []byte, str string) {
	if rw.wroteHeader {
		return
	}
	if len(str) > 512 {
		str = str[:512]
	}

	m := rw.Header()

	_, hasType := m["Content-Type"]
	hasTE := m.Get("Transfer-Encoding") != ""
	if !hasType && !hasTE {
		if b == nil {
			b = []byte(str)
		}
		m.Set("Content-Type", http.DetectContentType(b))
	}

	rw.WriteHeader(200)
}

// Write is the io.Writer interface with the modification required
// To wait the body. This will wait until SetBody is called with
// The desired storage.
func (rw *Response) Write(buf []byte) (int, error) {
	if !rw.wroteHeader {
		rw.writeHeader(buf, "")
	}

	if !rw.firstByteSent {
		rw.firstByteSent = true
		rw.WaitBody()
	}

	if rw.body != nil {
		return rw.body.Write(buf)
	}

	return 0, errors.New("No storage")
}

// WaitClose blocks until Close is called
func (rw *Response) WaitClose() {
	rw.closedLock.RLock()
}

// WaitBody blocks until body is Set
func (rw *Response) WaitBody() {
	rw.bodyLock.RLock()
}

// WaitHeaders blocks until headers are sent
func (rw *Response) WaitHeaders() {
	rw.headersLock.RLock()
}

func (rw *Response) SetBody(body storage.ResponseStorage) {
	rw.body = body
	rw.bodyLock.Unlock()
}

func (rw *Response) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.Code = code
	rw.wroteHeader = true

	rw.snapHeader = http.Header{}
	copyHeaders(rw.Header(), rw.snapHeader)
	rw.headersLock.Unlock()
}

func (rw *Response) Flush() {
	if !rw.wroteHeader {
		rw.WriteHeader(200)
	}

	if rw.body == nil {
		return
	}

	rw.body.Flush()
	return
}

// Close means there won't be any more Writes
// It closes body if it was set before
// It should be called after SetBody using WaitBody()
// Otherwise body won't be closed blocking the response
func (rw *Response) Close() error {
	defer rw.closedLock.Unlock()

	if rw.body != nil {
		return rw.body.Close()
	}
	return nil
}

// Clean the body if it is set
func (rw *Response) Clean() error {
	if rw.body == nil {
		return nil
	}

	return rw.body.Clean()
}
