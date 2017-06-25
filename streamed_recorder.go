package cache

import (
	"io"
	"net/http"
)

type EventHandler interface {
	onFirstByte(code int, headerMap http.Header) (io.Writer, error)
	onFlush()
	onWrite()
}

// StreamedRecorder is really similar to http.httpRecorder
// But this implementation allows to save the content while is being sent
// It also exposes a header chan that will send the headers
// when the first byte is sent to allow swapping the body storage
// If body is nil it gets discarded
type StreamedRecorder struct {
	Code      int         // the HTTP response code from WriteHeader
	HeaderMap http.Header // the HTTP response headers
	Body      io.Writer

	// Flushed is whether the Handler called Flush.
	Flushed bool

	eventHandler EventHandler

	result      *http.Response // cache of Result's return value
	snapHeader  http.Header    // snapshot of HeaderMap at first Write
	wroteHeader bool
}

// NewStreamedRecorder returns an initialized StreamedRecorder.
func NewStreamedRecorder(eventHandler EventHandler) *StreamedRecorder {
	return &StreamedRecorder{
		HeaderMap:    http.Header{},
		Body:         nil,
		Code:         200,
		eventHandler: eventHandler,
	}
}

// Header returns the response headers.
func (rw *StreamedRecorder) Header() http.Header {
	return rw.HeaderMap
}

// TODO
// // fix StreamRecorder is not CloseNotify
// func (rw *StreamedRecorder) CloseNotify() <-chan bool {
// 	return rw.w.(http.CloseNotifier).CloseNotify()
// }

// writeHeader writes a header if it was not written yet and
// detects Content-Type if needed.
//
// bytes or str are the beginning of the response body.
// We pass both to avoid unnecessarily generate garbage
// in rw.WriteString which was created for performance reasons.
// Non-nil bytes win.
func (rw *StreamedRecorder) writeHeader(b []byte, str string) {
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

// Write always succeeds and writes to rw.Body, if not nil.
func (rw *StreamedRecorder) Write(buf []byte) (int, error) {
	if !rw.wroteHeader {
		rw.writeHeader(buf, "")
	}

	rw.eventHandler.onWrite()

	if rw.Body != nil {
		return rw.Body.Write(buf)
	}

	return len(buf), nil
}

// WriteHeader sets rw.Code. After it is called, changing rw.Header
// will not affect rw.HeaderMap.
func (rw *StreamedRecorder) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.Code = code
	rw.wroteHeader = true
	rw.WriteHeader(code)

	newBody, err := rw.eventHandler.onFirstByte(code, cloneHeader(rw.HeaderMap))
	if err == nil {
		rw.Body = newBody
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
func (rw *StreamedRecorder) Flush() {
	if !rw.wroteHeader {
		rw.WriteHeader(200)
	}

	rw.eventHandler.onFlush()
	// if f, ok := rw.w.(http.Flusher); ok {
	// f.Flush()
	// }
	rw.Flushed = true
}
