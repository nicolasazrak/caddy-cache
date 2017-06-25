package cache

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

type UpstreamHandler struct {
	Next       httpserver.Handler
	f          *os.File
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
	fmt.Println("first byte sent, creating file")
	f, err := ioutil.TempFile("", "")

	if err != nil {
		fmt.Println("err", err)
		handler.resultChan <- UpstreamResult{
			err: err,
		}
		return nil, err
	}

	fmt.Println("File", f.Name())
	handler.entry.Response = &Response{
		Code:         code,
		HeaderMap:    headerMap,
		BodyFilePath: f.Name(),
	}

	fmt.Println("Sending over the chanel the upstream result")
	handler.resultChan <- UpstreamResult{
		entry: handler.entry,
	}

	fmt.Println("Swapping body")
	return f, nil
}

func (handler *UpstreamHandler) onFlush() {
	handler.entry.notifySubscribers()
}

func (handler *UpstreamHandler) onWrite() {
	if handler.f != nil {
		handler.f.Sync()
	}
	handler.entry.notifySubscribers()
}

func (handler *UpstreamHandler) onEnd() {
	if handler.f != nil {
		handler.f.Sync()
		handler.f.Close()
	}
	fmt.Println("Response ended, closing entry")
	handler.entry.close()
}

// Fetches upstream and returns a channel that will send the httpCacheEntry
// As soon as all headers are sent (when the first body byte is sent)
func (handler *UpstreamHandler) FetchUpstream() <-chan UpstreamResult {

	go func() {

		fmt.Println("Creating NewStreamedRecorder")

		streamRecorder := NewStreamedRecorder(handler)

		fmt.Println("ServeHTTP to upstream")
		handler.Next.ServeHTTP(streamRecorder, handler.r)
		handler.onEnd()

	}()

	return handler.resultChan
}

func NewUpstreamHandler(Next httpserver.Handler, r *http.Request) *UpstreamHandler {
	return &UpstreamHandler{
		r:          r,
		Next:       Next,
		resultChan: make(chan UpstreamResult),
		entry:      &HttpCacheEntry{},
	}
}
