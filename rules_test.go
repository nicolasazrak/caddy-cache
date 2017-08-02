package cache

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func makeRequest(URL string, headers http.Header) *http.Request {
	r, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		panic(err)
	}

	copyHeaders(headers, r.Header)

	return r
}

func makeResponse(code int, headers http.Header) *Response {
	return &Response{
		Code:       code,
		snapHeader: headers,
	}
}

func makeHeader(key string, value string) http.Header {
	h := http.Header{}
	h.Add(key, value)
	return h
}

func TestCacheableStatus(t *testing.T) {

	c := emptyConfig()
	c.DefaultMaxAge = time.Duration(1) * time.Second
	c.LockTimeout = time.Duration(5) * time.Hour
	c.CacheRules = []CacheRule{
		&PathCacheRule{Path: "/public"},
	}
	testTime := time.Now()
	now = func() time.Time {
		return testTime
	}

	t.Run("it should handle parsing error", func(t *testing.T) {
		request := makeRequest("/", http.Header{})
		response := makeResponse(200, makeHeader("Cache-Control", "max-age=ss"))
		isPublic, expiration := getCacheableStatus(request, response, c)

		require.False(t, isPublic)
		require.Equal(t, time.Time{}, expiration)
	})

	t.Run("it should return lockTimeout if response is private", func(t *testing.T) {
		request := makeRequest("/", http.Header{})
		response := makeResponse(200, makeHeader("Cache-control", "private"))
		isPublic, expiration := getCacheableStatus(request, response, c)

		require.False(t, isPublic)
		require.Equal(t, testTime.Add(c.LockTimeout), expiration)
	})

	t.Run("it should return lockTimeout if response has Vary: *", func(t *testing.T) {
		request := makeRequest("/", http.Header{})
		response := makeResponse(200, makeHeader("Vary", "*"))
		isPublic, expiration := getCacheableStatus(request, response, c)

		require.False(t, isPublic)
		require.Equal(t, testTime.Add(c.LockTimeout), expiration)
	})

	t.Run("should return public = false if does not have explicit expiration", func(t *testing.T) {
		request := makeRequest("/", http.Header{})
		response := makeResponse(200, http.Header{})
		isPublic, expiration := getCacheableStatus(request, response, c)

		require.False(t, isPublic)
		require.Equal(t, testTime.Add(c.LockTimeout), expiration)
	})

	t.Run("should return public = true if it has explicit expiration", func(t *testing.T) {
		request := makeRequest("/", http.Header{})
		response := makeResponse(200, makeHeader("Cache-control", "max-age=5"))
		isPublic, expiration := getCacheableStatus(request, response, c)

		require.True(t, isPublic)

		// Round is required because cachecontrol library uses time.Now() inside
		require.Equal(t, testTime.Add(time.Duration(5)*time.Second).UTC().Round(time.Second), expiration.UTC().Round(time.Second))
	})

	t.Run("should use default max age if rules matches and no expiration specified", func(t *testing.T) {
		request := makeRequest("/public", http.Header{})
		response := makeResponse(200, http.Header{})
		isPublic, expiration := getCacheableStatus(request, response, c)

		require.True(t, isPublic)
		require.Equal(t, testTime.Add(c.DefaultMaxAge), expiration)
	})

	t.Run("should use specified expiration if rules matches and expiration is set", func(t *testing.T) {
		request := makeRequest("/public", http.Header{})
		response := makeResponse(200, makeHeader("Cache-control", "max-age=50"))
		isPublic, expiration := getCacheableStatus(request, response, c)

		require.True(t, isPublic)

		// Round is required because cachecontrol library uses time.Now() inside
		require.Equal(t, testTime.Add(time.Duration(50)*time.Second).UTC().Round(time.Second), expiration.UTC().Round(time.Second))
	})
}

func TestHeaderCacheRule(t *testing.T) {
	r := &HeaderCacheRule{
		Header: "Content-Type",
		Value:  []string{"image/png", "image/jpg"},
	}

	t.Run("should match equal header and value", func(t *testing.T) {
		r := &HeaderCacheRule{
			Header: "Content-Type",
			Value:  []string{"image/jpg"},
		}

		matched := r.matches(makeRequest("/", http.Header{}), 200, makeHeader("Content-Type", "image/jpg"))
		require.True(t, matched)
	})

	t.Run("should match equal header and 1 value", func(t *testing.T) {
		matched := r.matches(makeRequest("/", http.Header{}), 200, makeHeader("Content-Type", "image/png"))
		require.True(t, matched)
	})

	t.Run("should not match different header but same", func(t *testing.T) {
		matched := r.matches(makeRequest("/", http.Header{}), 200, makeHeader("X-Custom-Header", "image/png"))
		require.False(t, matched)
	})
}
