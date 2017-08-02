package cache

import (
	"net/http"
	"strings"
	"time"

	"github.com/pquerna/cachecontrol/cacheobject"
)

// CacheRule determines if a request should be cached
type CacheRule interface {
	matches(*http.Request, int, http.Header) bool
}

// PathCacheRule matches if the request starts with given Path
type PathCacheRule struct {
	Path string
}

// HeaderCacheRule matches if given Header matches any of the values
type HeaderCacheRule struct {
	Header string
	Value  []string
}

// Made for testing
var now = time.Now

/* This rules decide if the request must be cached and are added to handler config if are present in Caddyfile */

func (rule *PathCacheRule) matches(req *http.Request, statusCode int, respHeaders http.Header) bool {
	return strings.HasPrefix(req.URL.Path, rule.Path)
}

func (rule *HeaderCacheRule) matches(req *http.Request, statusCode int, respHeaders http.Header) bool {
	headerValue := respHeaders.Get(rule.Header)
	for _, expectedValue := range rule.Value {
		if expectedValue == headerValue {
			return true
		}
	}
	return false
}

func getCacheableStatus(req *http.Request, response *Response, config *Config) (bool, time.Time) {
	// Partial responses are not supported yet
	if response.Code == http.StatusPartialContent || response.snapHeader.Get("Content-Range") != "" {
		return false, now().Add(config.LockTimeout)
	}

	reasonsNotToCache, expiration, err := cacheobject.UsingRequestResponse(req, response.Code, response.snapHeader, false)

	// err means there was an error parsing headers
	// Just ignore them and make response not cacheable
	if err != nil {
		return false, time.Time{}
	}

	isPublic := len(reasonsNotToCache) == 0

	if !isPublic {
		return false, now().Add(config.LockTimeout)
	}

	varyHeader := response.HeaderMap.Get("Vary")
	if varyHeader == "*" {
		return false, now().Add(config.LockTimeout)
	}

	// Check if any rule matches
	for _, rule := range config.CacheRules {
		if rule.matches(req, response.Code, response.snapHeader) {

			// If any rule matches but the response has no explicit expiration
			if expiration.Before(now()) {
				// Use the default max age
				expiration = now().Add(config.DefaultMaxAge)
			}
			return true, expiration
		}
	}

	// isPublic only if has an explicit expiration
	if expiration.Before(now()) {
		return false, now().Add(config.LockTimeout)
	}

	return true, expiration
}

func matchesVary(currentRequest *http.Request, entry *HTTPCacheEntry) bool {
	vary := entry.Response.HeaderMap.Get("Vary")

	for _, searchedHeader := range strings.Split(vary, ",") {
		searchedHeader = strings.TrimSpace(searchedHeader)
		if currentRequest.Header.Get(searchedHeader) != entry.Request.Header.Get(searchedHeader) {
			return false
		}
	}

	return true
}
