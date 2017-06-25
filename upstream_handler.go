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
		handler.resultChan <- UpstreamResult{
			err: err,
		}
		return nil, err
	}

	writer := handler.entry.GetBodyWriter()

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
	fmt.Println("Response ended, closing entry")
	handler.entry.Close()
}

// Fetches upstream and returns a channel that will send the httpCacheEntry
// As soon as all headers are sent (when the first body byte is sent)
func (handler *UpstreamHandler) doRequest() <-chan UpstreamResult {
	go func() {
		streamRecorder := NewStreamedRecorder(handler)
		handler.Next.ServeHTTP(streamRecorder, handler.r)
		handler.onEnd()
	}()

	return handler.resultChan
}

func FetchUpstream(Next httpserver.Handler, r *http.Request) <-chan UpstreamResult {
	h := &UpstreamHandler{
		r:          r,
		Next:       Next,
		resultChan: make(chan UpstreamResult),
		entry: &HttpCacheEntry{
			Request: r,
		},
	}
	return h.doRequest()
}
