package cache

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

type CacheRule interface {
	matches(*http.Request, *httptest.ResponseRecorder) bool
}

type PathCacheRule struct {
	Path string
}

type HeaderCacheRule struct {
	Header string
	Value  []string
}

/* This rules decide if the request must be cached and are added to handler config if are present in Caddyfile */

func (rule *PathCacheRule) matches(req *http.Request, res *httptest.ResponseRecorder) bool {
	return strings.HasPrefix(req.URL.Path, rule.Path)
}

func (rule *HeaderCacheRule) matches(req *http.Request, res *httptest.ResponseRecorder) bool {
	headerValue := res.HeaderMap.Get(rule.Header)
	for _, expectedValue := range rule.Value {
		if expectedValue == headerValue {
			return true
		}
	}
	return false
}
