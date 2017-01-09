package cache

import (
	"github.com/pquerna/cachecontrol"
	"net/http"
	"strings"
	"time"
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

func shouldUseCache(req *http.Request) bool {
	// TODO Add more logic like get params, ?nocache=true

	if req.Method != "GET" && req.Method != "HEAD" {
		// Only cache Get and head request
		return false
	}

	// Range responses still not supported
	if req.Header.Get("accept-ranges") != "" {
		return false
	}

	return true
}

func getCacheableStatus(req *http.Request, res *StreamedRecorder, config *Config) (bool, time.Time, error) {
	reasonsNotToCache, expiration, err := cachecontrol.CachableResponse(req, res.Result(), cachecontrol.Options{})

	if err != nil {
		return false, time.Now(), err
	}

	canBeStored := len(reasonsNotToCache) == 0

	if !canBeStored {
		return false, time.Now(), nil
	}

	varyHeaders, ok := res.HeaderMap["Vary"]
	if ok && varyHeaders[0] == "*" {
		return false, time.Now(), nil
	}

	hasExplicitExpiration := expiration.After(time.Now().UTC())

	if expiration.Before(time.Now().UTC().Add(time.Duration(1) * time.Second)) {
		// If expiration is not specified or is before now use default MaxAge
		expiration = time.Now().UTC().Add(config.DefaultMaxAge)
	}

	anyCacheRulesMatches := false
	for _, rule := range config.CacheRules {
		if rule.matches(req, res) {
			anyCacheRulesMatches = true
			break
		}
	}

	if err != nil {
		return false, time.Now(), err
	}

	return anyCacheRulesMatches || hasExplicitExpiration, expiration, nil
}
