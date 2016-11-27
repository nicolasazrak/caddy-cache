package cache

import (
	"testing"
	"github.com/nicolasazrak/caddy-cache/storage"
	"net/http"
	"net/http/httptest"
	"github.com/stretchr/testify/assert"
	"net/url"
	"fmt"
)

type TestHandler struct {
	timesCalled int
	ResponseBody []byte
	ResponseCode int
	ResponseError error
	ResponseHeaders http.Header
}

/* Helpers */

func (h *TestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	h.timesCalled = h.timesCalled + 1
	for k, values := range h.ResponseHeaders {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(h.ResponseCode)
	w.Write(h.ResponseBody)
	return h.ResponseCode, h.ResponseError
}

func buildBasicHandler() (*CacheHandler, *TestHandler) {
	memory := storage.MemoryStorage{}
	memory.Setup()
	backend := TestHandler{
		ResponseCode: 200,
	}

	return &CacheHandler{
		Config: &Config {
			CacheRules: []CacheRule{},
			DefaultMaxAge: 10,
		},
		Client: &memory,
		Next: &backend,
	}, &backend
}

func buildGetRequestWithHeaders(path string, headers http.Header) *http.Request {
	reqUrl, err := url.Parse(path)
	if err != nil {
		panic(fmt.Sprintf("Invalid url %s in test", path))
	}
	return &http.Request{
		Method: "GET",
		URL: reqUrl,
		Header: headers,
	}
}

func buildGetRequest(path string) *http.Request {
	return buildGetRequestWithHeaders(path, http.Header{})
}

func makeNRequests(handler *CacheHandler, n int, req *http.Request) ([]*httptest.ResponseRecorder, error){
	responses := []*httptest.ResponseRecorder{}
	for i := 0; i < n; i++ {
		recorder := httptest.NewRecorder()
		_, err := handler.ServeHTTP(recorder, req)
		if err != nil {
			return nil, err
		}
		responses = append(responses, recorder)
	}
	return responses, nil
}

/* Actual Tests */


// TODO avoid code duplication, use r.Run

func TestCacheByCacheControlHeader(t *testing.T) {
	handler, backend := buildBasicHandler()
	backend.ResponseHeaders = http.Header{
		"Cache-control": []string { "public; max-age=3600" },
	}

	_, err := makeNRequests(handler, 2, buildGetRequest("http://somehost.com/"))
	assert.NoError(t, err, "Failed doing requests")

	assert.Equal(t, 1, backend.timesCalled, "Backend should have been called 1 but it was called", backend.timesCalled)
}

func TestCacheByExpiresHeader(t *testing.T) {
	handler, backend := buildBasicHandler()
	backend.ResponseHeaders = http.Header{
		"Expires": []string { "Thu, 01 Dec 2820 16:00:00 GMT" },
	}

	_, err := makeNRequests(handler, 2, buildGetRequest("http://somehost.com/"))
	assert.NoError(t, err, "Failed doing requests")

	assert.Equal(t, 1, backend.timesCalled, "Backend should have been called 1 but it was called", backend.timesCalled)
}

func TestNoCacheByExpiredHeader(t *testing.T) {
	handler, backend := buildBasicHandler()
	backend.ResponseHeaders = http.Header{
		"Expires": []string { "Thu, 01 Dec 1994 16:00:00 GMT" },
	}

	_, err := makeNRequests(handler, 2, buildGetRequest("http://somehost.com/"))
	assert.NoError(t, err, "Failed doing requests")

	assert.Equal(t, 2, backend.timesCalled, "Backend should have been called 1 but it was called", backend.timesCalled)
}

func TestCacheByPath(t *testing.T) {
	handler, backend := buildBasicHandler()
	handler.Config.CacheRules = append(handler.Config.CacheRules, &PathCacheRule{ Path: "/assets" })

	req := buildGetRequest("http://somehost.com/assets/1")

	_, err := makeNRequests(handler, 2, req)
	assert.NoError(t, err, "Failed doing requests")

	assert.Equal(t, 1, backend.timesCalled, "Backend should have been called 1 but it was called", backend.timesCalled)
}

func TestNotCacheablePath(t *testing.T) {
	handler, backend := buildBasicHandler()
	handler.Config.CacheRules = append(handler.Config.CacheRules, &PathCacheRule{ Path: "/assets" })

	req := buildGetRequest("http://somehost.com/api/1")

	_, err := makeNRequests(handler, 2, req)
	assert.NoError(t, err, "Failed doing requests")

	assert.Equal(t, 2, backend.timesCalled, "Backend should have been called 2 but it was called", backend.timesCalled)
}

func TestNotCacheableMethod(t *testing.T) {
	handler, backend := buildBasicHandler()
	handler.Config.CacheRules = append(handler.Config.CacheRules, &PathCacheRule{ Path: "/assets" })

	reqUrl, _ := url.Parse("http://somehost.com/assets/some.jpg")
	req := &http.Request{
		Method: "POST",
		URL: reqUrl,
	}

	_, err := makeNRequests(handler, 2, req)
	assert.NoError(t, err, "Failed doing requests")

	assert.Equal(t, 2, backend.timesCalled, "Backend should have been called 2 but it was called", backend.timesCalled)
}

func TestNotCacheableCacheControl(t *testing.T) {
	handler, backend := buildBasicHandler()
	handler.Config.CacheRules = append(handler.Config.CacheRules, &PathCacheRule{ Path: "/assets" })

	backend.ResponseHeaders = http.Header{
		"Cache-control": []string { "private" },
	}

	_, err := makeNRequests(handler, 2, buildGetRequest("http://somehost.com/assets/1"))
	assert.NoError(t, err, "Failed doing requests")

	assert.Equal(t, 2, backend.timesCalled, "Backend should have been called 2 but it was called", backend.timesCalled)
}

func TestAddAllHeaders(t *testing.T) {
	handler, backend := buildBasicHandler()

	responseHeaders := http.Header{
		"Content-Type": []string { "text/plain; charset=utf-8" },
		"X-Custom-2": []string { "bar", "baz" },
		"X-Custom": []string { "foo", "bar", "baz" },
	}
	backend.ResponseHeaders = responseHeaders

	responses, err := makeNRequests(handler, 2, buildGetRequest("http://somehost.com/assets/1"))
	assert.NoError(t, err, "Failed doing requests")

	assert.Equal(t, responseHeaders, responses[0].HeaderMap, "Cache didn't send same headers that backend originally sent")
}

func TestCacheByHeaders(t *testing.T) {
	handler, backend := buildBasicHandler()

	handler.Config.CacheRules = append(handler.Config.CacheRules, &HeaderCacheRule{
		Header: "Content-Type",
		Value: []string { "image/png", "image/gif" },
	})

	// First requests with png images that should be cached
	backend.ResponseHeaders = http.Header{ "Content-Type": []string { "image/png" } }
	_, err := makeNRequests(handler, 5, buildGetRequest("http://somehost.com/another_not_cached_path/png"))
	assert.NoError(t, err, "Failed doing requests")
	assert.Equal(t, 1, backend.timesCalled, "Cache should have been called once, but it was called", backend.timesCalled)

	// Second requests with gifs that should also be cached
	backend.ResponseHeaders = http.Header{ "Content-Type": []string { "image/gif" } }
	_, err = makeNRequests(handler, 2, buildGetRequest("http://somehost.com/another_not_cached_path/gif"))
	assert.NoError(t, err, "Failed doing requests")
	assert.Equal(t, 2, backend.timesCalled, "Cache should have been called twice but is was called", backend.timesCalled)

	// Third request with videos that should also be cached
	backend.ResponseHeaders = http.Header{ "Content-Type": []string { "video/mp4" } }
	_, err = makeNRequests(handler, 10, buildGetRequest("http://somehost.com/another_not_cached_path/mp4"))
	assert.NoError(t, err, "Failed doing requests")
	assert.Equal(t, 12, backend.timesCalled, "Cache should have been called 12 times but was called", backend.timesCalled)
}


func TestVaryAll(t *testing.T) {
	handler, backend := buildBasicHandler()

	backend.ResponseHeaders = http.Header{
		"Vary": []string { "*" },
		"Cache-Control": []string { "max-age=3600" },
	}

	_, err := makeNRequests(handler, 2, buildGetRequest("http://somehost.com/assets/1"))
	assert.NoError(t, err, "Failed doing requests")

	assert.Equal(t, 2, backend.timesCalled, "Invalid number of times called")
}


func TestVaryAcceptEncoding(t *testing.T) {
	handler, backend := buildBasicHandler()

	backend.ResponseHeaders = http.Header{
		"Vary": []string { "Accept-Encoding" },
		"Cache-Control": []string { "max-age=3600" },
	}

	_, err := makeNRequests(handler, 2, buildGetRequestWithHeaders("http://somehost.com/assets/1", http.Header{
		"Accept-Encoding": { "gzip" },
	}))
	assert.NoError(t, err, "Failed doing requests")
	assert.Equal(t, 1, backend.timesCalled, "Invalid number of times called")

	_, err = makeNRequests(handler, 3, buildGetRequestWithHeaders("http://somehost.com/assets/1", http.Header{
		"Accept-Encoding": { "deflate" },
	}))
	assert.NoError(t, err, "Failed doing requests")
	assert.Equal(t, 2, backend.timesCalled, "Invalid number of times called")

}

func TestVaryWithTwoHeaders(t *testing.T) {
	handler, backend := buildBasicHandler()

	backend.ResponseHeaders = http.Header{
		"Vary": []string { "Accept-Encoding, User-Agent" },
		"Cache-Control": []string { "max-age=3600" },
	}

	_, err := makeNRequests(handler, 2, buildGetRequestWithHeaders("http://somehost.com/assets/1", http.Header{
		"Accept-Encoding": { "gzip" },
		"User-Agent": { "Mobile" },
		"X-Another": { "x" },
	}))
	assert.NoError(t, err, "Failed doing requests")
	assert.Equal(t, 1, backend.timesCalled, "Invalid number of times called")

	_, err = makeNRequests(handler, 2, buildGetRequestWithHeaders("http://somehost.com/assets/1", http.Header{
		"Accept-Encoding": { "gzip" },
		"User-Agent": { "Mobile" },
		"X-Another": { "Y" },
	}))
	assert.NoError(t, err, "Failed doing requests")
	assert.Equal(t, 1, backend.timesCalled, "Invalid number of times called")

	_, err = makeNRequests(handler, 3, buildGetRequestWithHeaders("http://somehost.com/assets/1", http.Header{
		"Accept-Encoding": { "gzip" },
		"User-Agent": { "Desktop" },
		"X-Another": { "X" },
	}))
	assert.NoError(t, err, "Failed doing requests")
	assert.Equal(t, 2, backend.timesCalled, "Invalid number of times called")

}


func TestStatusCacheSkip(t *testing.T) {
	handler, _ := buildBasicHandler()
	handler.Config.StatusHeader = "cache-status"

	reqUrl, _ := url.Parse("http://somehost.com/assets/some.jpg")
	responses, err := makeNRequests(handler, 1, &http.Request{ Method: "POST", URL: reqUrl })
	assert.NoError(t, err, "Failed doing requests")

	assert.Equal(t, "skip", responses[0].HeaderMap.Get("cache-status"))
}

func TestStatusCacheHit(t *testing.T) {
	handler, backend := buildBasicHandler()
	handler.Config.StatusHeader = "cache-status"

	backend.ResponseHeaders = http.Header{
		"Cache-control": []string { "public; max-age=3600" },
	}

	responses, err := makeNRequests(handler, 1, buildGetRequest("http://somehost.com/"))
	assert.NoError(t, err, "Failed doing requests")
	assert.Equal(t, "miss", responses[0].HeaderMap.Get("cache-status"))

	responses, err = makeNRequests(handler, 1, buildGetRequest("http://somehost.com/"))
	assert.NoError(t, err, "Failed doing requests")
	assert.Equal(t, "hit", responses[0].HeaderMap.Get("cache-status"))
}


func TestDefaultCacheTime(t *testing.T) {
	// TODO test this
	// isCacheable, expiration := getCacheableStatus(req, res, config)
}
