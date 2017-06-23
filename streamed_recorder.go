package cache

import (
	"net/http"
)

/**
* Streamed Recorder is really similar to http.httpRecorder
* But this implementation is created with a ResponseWriter
* And sends the response to downstream at the same time
* the response is being recorded avoiding having to wait to
* have all the response.
 */

type StreamedRecorder struct {
	Code      int         // the HTTP response code from WriteHeader
	HeaderMap http.Header // the HTTP response headers
	Body      StorageContent
	Flushed   bool

	result      *http.Response // cache of Result's return value
	snapHeader  http.Header    // snapshot of HeaderMap at first Write
	wroteHeader bool

	w http.ResponseWriter // This is the downstream

	calledWriteListener bool
	firstWriteListener  func(int, http.Header) error
}

// NewRecorder returns an initialized StreamedRecorder.
func NewStreamedRecorder(w http.ResponseWriter) *StreamedRecorder {
	return &StreamedRecorder{
		HeaderMap:           w.Header(),
		Body:                nil,
		Code:                200,
		firstWriteListener:  nil,
		w:                   w,
		calledWriteListener: false,
	}
}

// DefaultRemoteAddr is the default remote address to return in RemoteAddr if
// an explicit DefaultRemoteAddr isn't set on StreamedRecorder.
const DefaultRemoteAddr = "1.2.3.4"

// Header returns the response headers.
func (rw *StreamedRecorder) Header() http.Header {
	return rw.w.Header()
}

// fix StreamRecorder is not CloseNotify
func (rw *StreamedRecorder) CloseNotify() <-chan bool {
	 return rw.w.(http.CloseNotifier).CloseNotify()
}


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

	if !rw.calledWriteListener {
		rw.calledWriteListener = true
		rw.firstWriteListener(rw.Code, rw.snapHeader)
	}

	if rw.Body != nil {
		rw.Body.Write(buf)
	}

	return rw.w.Write(buf)
}

func (rw *StreamedRecorder) UpdateBodyWriter(bw StorageContent) {
	rw.Body = bw
}

// WriteHeader sets rw.Code. After it is called, changing rw.Header
// will not affect rw.HeaderMap.
func (rw *StreamedRecorder) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.Code = code
	rw.wroteHeader = true
	rw.snapHeader = cloneHeader(rw.w.Header())
	rw.w.WriteHeader(code)
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

	if f, ok := rw.w.(http.Flusher); ok {
		f.Flush()
	}
	rw.Flushed = true
}

func (rw *StreamedRecorder) SetFirstWriteListener(fn func(int, http.Header) error) {
	rw.firstWriteListener = fn
}

// Result returns the response generated by the handler.
//
// The returned Response will have at least its StatusCode,
// Header and optionally Body populated.
// More fields may be populated in the future, so callers should
// not DeepEqual the result in tests.
//
// The Response.Header is a snapshot of the headers at the time of the
// first write call, or at the time of this call, if the handler never
// did a write.
//
// Result must only be called after the handler has finished running.
func (rw *StreamedRecorder) Result() (*http.Response, StorageContent) {
	if rw.result != nil {
		return rw.result, nil
	}
	if rw.snapHeader == nil {
		rw.snapHeader = cloneHeader(rw.w.Header())
	}
	res := &http.Response{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: rw.Code,
		Header:     rw.snapHeader,
	}
	rw.result = res
	if res.StatusCode == 0 {
		res.StatusCode = 200
	}
	res.Status = http.StatusText(res.StatusCode)

	return res, rw.Body
}
