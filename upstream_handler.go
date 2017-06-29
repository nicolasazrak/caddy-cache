package cache

import (
	"fmt"
	"io"
	"net/http"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

type UpstreamHandler struct {
	Next       httpserver.Handler
	r          *http.Request
	result     *UpstreamResult
	entry      *HttpCacheEntry
	resultChan chan UpstreamResult
	resultSent bool
	recording  *StreamedRecorder
}

type UpstreamResult struct {
	entry *HttpCacheEntry
	err   error
}

func (handler *UpstreamHandler) onFirstByte(code int, headerMap http.Header) (io.Writer, error) {
	err := handler.entry.UpdateResponse(&Response{
		Code:      code,
		HeaderMap: headerMap,
	})

	if err != nil {
		fmt.Println(err)
		handler.resultSent = true
		handler.resultChan <- UpstreamResult{
			err: err,
		}
		return nil, err
	}

	writer := handler.entry.GetBodyWriter()

	handler.resultSent = true
	handler.resultChan <- UpstreamResult{
		entry: handler.entry,
	}

	return writer, nil
}

func (handler *UpstreamHandler) onFlush() {
	handler.entry.OnFlush()
}

func (handler *UpstreamHandler) onWrite() {
	handler.entry.OnWrite()
}

func (handler *UpstreamHandler) onEnd() {
	if !handler.resultSent {
		handler.entry.UpdateResponse(&Response{
			Code:      handler.recording.Code,
			HeaderMap: handler.recording.HeaderMap,
		})
		handler.resultChan <- UpstreamResult{
			entry: handler.entry,
		}
	}

	// TODO handle error
	handler.entry.Close()
}

// Fetches upstream and returns a channel that will send the httpCacheEntry
// As soon as all headers are sent (when the first body byte is sent)
func (handler *UpstreamHandler) doRequest() <-chan UpstreamResult {
	go func() {
		handler.recording = NewStreamedRecorder(handler)
		handler.Next.ServeHTTP(handler.recording, handler.r)
		handler.onEnd()
	}()

	return handler.resultChan
}

// FetchUpstream fetchs upstream and returns a channel that will emit an UpstreamResult
func FetchUpstream(Next httpserver.Handler, r *http.Request) <-chan UpstreamResult {
	h := &UpstreamHandler{
		r:          r,
		Next:       Next,
		resultChan: make(chan UpstreamResult),
		entry:      NewHTTPCacheEntry(r),
	}
	return h.doRequest()
}
