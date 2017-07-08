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

func doRequest(t *testing.T, h httpserver.Handler) (*http.Response, error) {
	w := httptest.NewRecorder()
	r, urlErr := http.NewRequest("GET", "/", nil)
	require.NoError(t, urlErr)

	_, err := h.ServeHTTP(w, r)
	return w.Result(), err
}

func requireStatus(t *testing.T, r *http.Response, status string) {
	require.Equal(t, status, r.Header.Get("X-cache-status"))
}

func requireCode(t *testing.T, r *http.Response, expectedCode int) {
	require.Equal(t, expectedCode, r.StatusCode)
}

func requireBody(t *testing.T, r *http.Response, expectedBody []byte) {
	actualBody, err := ioutil.ReadAll(r.Body)
	require.NoError(t, err)
	require.Equal(t, expectedBody, actualBody)
}

func requestAndAssert(t *testing.T, h httpserver.Handler, expectedCode int, expectedStatus string, expectedBody []byte) {
	response, _ := doRequest(t, h)
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
	}))

	requestAndAssert(t, h, 200, cacheMiss, content)
	requestAndAssert(t, h, 200, cacheHit, content)
	require.Equal(t, 1, hits)
}

func TestPublicResponseWithoutBody(t *testing.T) {
	hits := 0
	h := NewHandler(httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
		hits++
		w.Header().Add("Cache-control", "max-age=1")
		return 200, nil
	}))

	requestAndAssert(t, h, 200, cacheMiss, []byte{})
	requestAndAssert(t, h, 200, cacheHit, []byte{})
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
	}))

	requestAndAssert(t, h, 200, cacheMiss, content)
	requestAndAssert(t, h, 200, cacheSkip, content)
	require.Equal(t, 2, hits)
}

func TestPrivateResponseWithoutBody(t *testing.T) {
	hits := 0
	h := NewHandler(httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
		hits++
		w.Header().Add("Cache-control", "private")
		return 200, nil
	}))

	requestAndAssert(t, h, 200, cacheMiss, []byte{})
	requestAndAssert(t, h, 200, cacheSkip, []byte{})
	require.Equal(t, 2, hits)
}

func TestHandlerError(t *testing.T) {
	badGateway := errors.New("Bad gateway")
	h := NewHandler(httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
		return 503, badGateway
	}))

	_, err1 := doRequest(t, h)
	require.Equal(t, badGateway, err1)

	_, err2 := doRequest(t, h)
	require.Equal(t, badGateway, err2)
}
