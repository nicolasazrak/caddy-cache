package cache

import (
	"net/http"
	"strings"
)

type CacheRule interface {
	matches(*http.Request, *StreamedRecorder) bool
}

type PathCacheRule struct {
	Path string
}

type HeaderCacheRule struct {
	Header string
	Value  []string
}

/* This rules decide if the request must be cached and are added to handler config if are present in Caddyfile */

func (rule *PathCacheRule) matches(req *http.Request, res *StreamedRecorder) bool {
	return strings.HasPrefix(req.URL.Path, rule.Path)
}

func (rule *HeaderCacheRule) matches(req *http.Request, res *StreamedRecorder) bool {
	headerValue := res.HeaderMap.Get(rule.Header)
	for _, expectedValue := range rule.Value {
		if expectedValue == headerValue {
			return true
		}
	}
	return false
}
