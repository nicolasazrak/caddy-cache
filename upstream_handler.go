package cache

import (
	"fmt"
	"io"
	"net/http"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

type upstreamHandler struct {
	Next       httpserver.Handler
	result     *UpstreamResult
	entry      *HTTPCacheEntry
	resultChan chan UpstreamResult
	resultSent bool
	recording  *StreamedRecorder
}

// UpstreamResult represent the result of fetching upstream
type UpstreamResult struct {
	entry *HTTPCacheEntry
	err   error
}

func (handler *upstreamHandler) onFirstByte(code int, headerMap http.Header) (io.Writer, error) {
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

func (handler *upstreamHandler) onFlush() {
	handler.entry.OnFlush()
}

func (handler *upstreamHandler) onWrite() {
	handler.entry.OnWrite()
}

func (handler *upstreamHandler) onEnd() {
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
func (handler *upstreamHandler) doRequest(r *http.Request) <-chan UpstreamResult {
	go func() {
		handler.recording = NewStreamedRecorder(handler)
		handler.Next.ServeHTTP(handler.recording, r)
		handler.onEnd()
	}()

	return handler.resultChan
}

// FetchUpstream fetchs upstream and returns a channel that will emit an UpstreamResult
func FetchUpstream(Next httpserver.Handler, r *http.Request) <-chan UpstreamResult {
	h := &upstreamHandler{
		Next:       Next,
		resultChan: make(chan UpstreamResult, 1),
		entry:      NewHTTPCacheEntry(r),
	}
	return h.doRequest(r)
}
