package cache

import (
	"testing"
	"github.com/nicolasazrak/caddy-cache/storage"
	"net/http"
	"net/http/httptest"
	"github.com/stretchr/testify/assert"
	"net/url"
)

type TestHandler struct {
	timesCalled int
	ResponseBody []byte
	ResponseCode int
	ResponseError error
	ResponseHeaders map[string][]string
}

func (h *TestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	h.timesCalled = h.timesCalled + 1
	w.Write(h.ResponseBody)
	w.WriteHeader(h.ResponseCode)
	for k, values := range h.ResponseHeaders {
		for _, v := range values {
			w.Header().Set(k, v)
		}
	}
	return h.ResponseCode, h.ResponseError
}

func buildBasicHandler(cacheablePaths string) (*CacheHandler, *TestHandler) {
	memory := storage.MemoryStorage{}
	memory.Setup()
	backend := TestHandler{}

	return &CacheHandler{
		Config: &Config {
			CacheablePaths: []string{ cacheablePaths },
			DefaultMaxAge: 10,
		},
		Client: &memory,
		Next: &backend,
	}, &backend
}

func buildGetRequest(path string) *http.Request {
	reqUrl, _ := url.Parse(path)
	return &http.Request{
		Method: "GET",
		URL: reqUrl,
	}

}

func TestBasicCache(t *testing.T) {
	handler, backend := buildBasicHandler("/assets")
	rec := httptest.NewRecorder()

	req := buildGetRequest("http://somehost.com/assets/1")

	_, err1 := handler.ServeHTTP(rec, req)
	_, err2 := handler.ServeHTTP(rec, req)
	if err1 != nil || err2 != nil {
		assert.Fail(t, "Error processing request", err1, err2)
	}

	assert.Equal(t, 1, backend.timesCalled, "Backend should have been called 1 but it was called", backend.timesCalled)
}

func TestNotCacheablePath(t *testing.T) {
	handler, backend := buildBasicHandler("/assets")
	rec := httptest.NewRecorder()

	req := buildGetRequest("http://somehost.com/api/1")

	_, err1 := handler.ServeHTTP(rec, req)
	_, err2 := handler.ServeHTTP(rec, req)
	if err1 != nil || err2 != nil {
		assert.Fail(t, "Error processing request", err1, err2)
	}

	assert.Equal(t, 2, backend.timesCalled, "Backend should have been called 2 but it was called", backend.timesCalled)
}

func TestNotCacheableMethod(t *testing.T) {
	handler, backend := buildBasicHandler("/assets")
	rec := httptest.NewRecorder()

	reqUrl, _ := url.Parse("http://somehost.com/assets/some.jpg")
	req := &http.Request{
		Method: "POST",
		URL: reqUrl,
	}

	_, err1 := handler.ServeHTTP(rec, req)
	_, err2 := handler.ServeHTTP(rec, req)
	if err1 != nil || err2 != nil {
		assert.Fail(t, "Error processing request", err1, err2)
	}

	assert.Equal(t, 2, backend.timesCalled, "Backend should have been called 2 but it was called", backend.timesCalled)
}

func TestNotCacheableCacheControl(t *testing.T) {
	handler, backend := buildBasicHandler("/assets")
	rec := httptest.NewRecorder()

	responseHeaders := make(map[string][]string)
	responseHeaders["Cache-control"] = []string { "private" }
	backend.ResponseHeaders = responseHeaders

	req := buildGetRequest("http://somehost.com/assets/1")

	_, err1 := handler.ServeHTTP(rec, req)
	_, err2 := handler.ServeHTTP(rec, req)
	if err1 != nil || err2 != nil {
		assert.Fail(t, "Error processing request", err1, err2)
	}

	assert.Equal(t, 2, backend.timesCalled, "Backend should have been called 2 but it was called", backend.timesCalled)
}

func TestDefaultCacheTime(t *testing.T) {
	// TODO test this
	// isCacheable, expiration := getCacheableStatus(req, res, config)
}
