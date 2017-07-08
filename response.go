package cache

import (
	"errors"
	"net/http"
	"sync"
)

type Response struct {
	Code      int         // the HTTP response code from WriteHeader
	HeaderMap http.Header // the HTTP response headers
	body      ResponseStorage

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

func (rw *Response) WaitClose() {
	rw.closedLock.RLock()
}

func (rw *Response) WaitBody() {
	rw.bodyLock.RLock()
}

func (rw *Response) WaitHeaders() {
	rw.headersLock.RLock()
}

func (rw *Response) SetBody(body ResponseStorage) {
	rw.body = body
	rw.bodyLock.Unlock()
}

func (rw *Response) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.Code = code
	rw.wroteHeader = true
	rw.headersLock.Unlock()
}

func (rw *Response) Flush() error {
	if !rw.wroteHeader {
		rw.WriteHeader(200)
	}

	if rw.body == nil {
		return nil
	}

	return rw.body.Flush()
}

func (rw *Response) Close() error {
	defer rw.closedLock.Unlock()

	if rw.body != nil {
		return rw.body.Close()
	}
	return nil
}

func (rw *Response) Clean() error {
	if rw.body == nil {
		return nil
	}

	return rw.body.Clean()
}
