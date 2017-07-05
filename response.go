package cache

import (
	"io"
	"net/http"
	"time"
)

type Response struct {
	isPublic   bool
	expiration time.Time

	Code      int         // the HTTP response code from WriteHeader
	HeaderMap http.Header // the HTTP response headers
	body      ResponseStorage

	wroteHeader    bool
	firstByteSent  bool
	storageBuilder *StorageBuilder

	errorChan   chan error
	headersChan chan http.Header
}

// NewResponse returns an initialized Response.
func NewResponse(buildStorage *StorageBuilder) *Response {
	return &Response{
		isPublic:       false,
		expiration:     time.Now(),
		Code:           200,
		HeaderMap:      http.Header{},
		body:           nil,
		storageBuilder: buildStorage,
		errorChan:      make(chan error, 1),
		headersChan:    make(chan http.Header, 1),
	}
}

// Header returns the response headers.
func (rw *Response) Header() http.Header {
	return rw.HeaderMap
}

// TODO
// // fix StreamRecorder is not CloseNotify
// func (rw *Response) CloseNotify() <-chan bool {
// 	return rw.w.(http.CloseNotifier).CloseNotify()
// }

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

func (rw *Response) handleFirstByte() error {
	// Prevent blocking
	select {
	case rw.headersChan <- rw.HeaderMap:
	default:
	}

	isPublic, expiration, newBody, err := rw.storageBuilder.BuildStorage(rw)
	if err != nil {
		select {
		case rw.errorChan <- err:
		default:
		}
		return err
	}

	rw.isPublic = isPublic
	rw.expiration = expiration
	rw.body = newBody
	return nil
}

// Write always succeeds and writes to rw.Body, if not nil.
func (rw *Response) Write(buf []byte) (int, error) {
	if !rw.firstByteSent {
		rw.firstByteSent = true
		err := rw.handleFirstByte()
		if err != nil {
			return 0, err
		}
	}

	if !rw.wroteHeader {
		rw.writeHeader(buf, "")
	}

	return rw.body.Write(buf)
}

// WriteHeader sets rw.Code. After it is called, changing rw.Header
// will not affect rw.HeaderMap.
func (rw *Response) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.Code = code
	rw.wroteHeader = true
}

func (rw *Response) replaceCode(code int) {
	if !rw.firstByteSent {
		rw.Code = code
	}
}

func cloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

// Flush sets rw.Flushed to true.
func (rw *Response) Flush() {
	if !rw.wroteHeader {
		rw.WriteHeader(200)
	}

	if rw.body != nil {
		rw.body.Flush()
	}
}

func (rw *Response) GetReader() (io.ReadCloser, error) {
	if rw.body == nil {
		return nil, nil
	}
	return rw.body.GetReader()
}

// Close the entry meaning the the response has ended
func (rw *Response) Close() error {
	if rw.body == nil {
		return nil
	}

	return rw.body.Close()
}

func (rw *Response) Clean() error {
	if rw.body != nil {
		return rw.body.Clean()
	}
	return nil
}
