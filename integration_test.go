package cache

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"io/ioutil"

	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/stretchr/testify/require"
)

func doRequestWithHeaders(t *testing.T, h httpserver.Handler, headers http.Header) (*http.Response, error) {
	w := httptest.NewRecorder()
	r, urlErr := http.NewRequest("GET", "/", nil)
	require.NoError(t, urlErr)

	r.Header = headers

	_, err := h.ServeHTTP(w, r)
	return w.Result(), err
}

func doRequestTo(t *testing.T, to string, h httpserver.Handler) (*http.Response, error) {
	w := httptest.NewRecorder()
	r, urlErr := http.NewRequest("GET", to, nil)
	require.NoError(t, urlErr)

	_, err := h.ServeHTTP(w, r)
	return w.Result(), err
}

func doRequest(t *testing.T, h httpserver.Handler) (*http.Response, error) {
	return doRequestWithHeaders(t, h, http.Header{})
}

func requireStatus(t *testing.T, r *http.Response, status string) {
	require.Equal(t, status, r.Header.Get(defaultStatusHeader))
}

func requireCode(t *testing.T, r *http.Response, expectedCode int) {
	require.Equal(t, expectedCode, r.StatusCode)
}

func requireBody(t *testing.T, r *http.Response, expectedBody []byte) {
	actualBody, err := ioutil.ReadAll(r.Body)
	require.NoError(t, err)
	require.Equal(t, expectedBody, actualBody)
}

func requestAndAssert(t *testing.T, h httpserver.Handler, headers http.Header, expectedCode int, expectedStatus string, expectedBody []byte) {
	response, _ := doRequestWithHeaders(t, h, headers)
	requireCode(t, response, expectedCode)
	requireStatus(t, response, expectedStatus)
	requireBody(t, response, expectedBody)
}

func TestPublicResponseWithBody(t *testing.T) {
	content := []byte("abc")
	hits := 0
	h := NewHandler(httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
		hits++
		w.Header().Add("Cache-control", "max-age=1")
		w.Write(content)
		return 200, nil
	}), emptyConfig())

	requestAndAssert(t, h, http.Header{}, 200, cacheMiss, content)
	requestAndAssert(t, h, http.Header{}, 200, cacheHit, content)
	require.Equal(t, 1, hits)
}

func TestPublicResponseWithoutBody(t *testing.T) {
	hits := 0
	h := NewHandler(httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
		hits++
		w.Header().Add("Cache-control", "max-age=1")
		return 200, nil
	}), emptyConfig())

	requestAndAssert(t, h, http.Header{}, 200, cacheMiss, []byte{})
	requestAndAssert(t, h, http.Header{}, 200, cacheHit, []byte{})
	require.Equal(t, 1, hits)
}

func TestPrivateResponseWithBody(t *testing.T) {
	content := []byte("abc")
	hits := 0
	h := NewHandler(httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
		hits++
		w.Header().Add("Cache-control", "private")
		w.Write(content)
		return 200, nil
	}), emptyConfig())

	requestAndAssert(t, h, http.Header{}, 200, cacheMiss, content)
	requestAndAssert(t, h, http.Header{}, 200, cacheSkip, content)
	require.Equal(t, 2, hits)
}

func TestPrivateResponseWithoutBody(t *testing.T) {
	hits := 0
	h := NewHandler(httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
		hits++
		w.Header().Add("Cache-control", "private")
		return 200, nil
	}), emptyConfig())

	requestAndAssert(t, h, http.Header{}, 200, cacheMiss, []byte{})
	requestAndAssert(t, h, http.Header{}, 200, cacheSkip, []byte{})
	require.Equal(t, 2, hits)
}

func TestHandlerError(t *testing.T) {
	badGateway := errors.New("Bad gateway")
	h := NewHandler(httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
		return 503, badGateway
	}), emptyConfig())

	_, err1 := doRequest(t, h)
	require.Equal(t, badGateway, err1)

	_, err2 := doRequest(t, h)
	require.Equal(t, badGateway, err2)
}

func TestPublicResponseWithVaryHeader(t *testing.T) {
	content := []byte("abc")
	hits := 0
	h := NewHandler(httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
		hits++
		w.Header().Add("Cache-control", "max-age=10")
		w.Header().Add("Vary", "Accept-Encoding")
		w.Write(content)
		return 200, nil
	}), emptyConfig())

	gzip := http.Header{"Accept-Encoding": []string{"gzip"}}
	requestAndAssert(t, h, gzip, 200, cacheMiss, content)
	requestAndAssert(t, h, gzip, 200, cacheHit, content)
	requestAndAssert(t, h, gzip, 200, cacheHit, content)
	require.Equal(t, 1, hits)

	deflate := http.Header{"Accept-Encoding": []string{"deflate"}}
	requestAndAssert(t, h, deflate, 200, cacheMiss, content)
	requestAndAssert(t, h, deflate, 200, cacheHit, content)
	requestAndAssert(t, h, deflate, 200, cacheHit, content)
	require.Equal(t, 2, hits)

	both := http.Header{"Accept-Encoding": []string{"gzip,defalte"}}
	requestAndAssert(t, h, both, 200, cacheMiss, content)
	requestAndAssert(t, h, both, 200, cacheHit, content)
	requestAndAssert(t, h, both, 200, cacheHit, content)
	require.Equal(t, 3, hits)
}

func TestConfigRules(t *testing.T) {
	content := []byte("abc")
	config := emptyConfig()
	config.CacheRules = []CacheRule{
		&PathCacheRule{Path: "/assets"},
		&HeaderCacheRule{Header: "X-Custom-header", Value: []string{"a", "b"}},
	}

	t.Run("path", func(t *testing.T) {
		hits := 0

		h := NewHandler(httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
			hits++
			w.Write(content)
			return 200, nil
		}), config)

		doRequestTo(t, "/private", h)
		doRequestTo(t, "/private", h)
		require.Equal(t, 2, hits)

		doRequestTo(t, "/assets", h)
		doRequestTo(t, "/assets", h)
		require.Equal(t, 3, hits)
	})

	t.Run("header matched", func(t *testing.T) {
		hits := 0

		h := NewHandler(httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
			hits++
			w.Header().Add("X-Custom-header", "a")
			w.Write(content)
			return 200, nil
		}), config)

		doRequest(t, h)
		doRequest(t, h)
		require.Equal(t, 1, hits)
	})

	t.Run("header not matched", func(t *testing.T) {
		hits := 0

		h := NewHandler(httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
			hits++
			w.Header().Add("X-Custom-header", "c")
			w.Write(content)
			return 200, nil
		}), config)

		doRequest(t, h)
		doRequest(t, h)
		require.Equal(t, 2, hits)
	})
}
