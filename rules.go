package cache

import (
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/pquerna/cachecontrol/cacheobject"
)

func getCacheableStatus(r *http.Request, statusCode int, respHeaders http.Header) (bool, time.Time, error) {
	reasonsNotToCache, expiration, err := cacheobject.UsingRequestResponse(r, statusCode, respHeaders, false)

	if err != nil {
		return false, time.Now(), err
	}

	canBeStored := len(reasonsNotToCache) == 0

	if !canBeStored {
		return false, time.Now(), nil
	}

	varyHeaders, ok := respHeaders["Vary"]
	if ok && varyHeaders[0] == "*" {
		return false, time.Now(), nil
	}

	return expiration.After(time.Now().UTC()), expiration, nil
}

func matchesVary(r *http.Request, previousEntry *HttpCacheEntry) bool {
	vary, hasVary := previousEntry.Response.HeaderMap["Vary"]
	if !hasVary {
		return true
	}

	for _, searchedHeader := range strings.Split(vary[0], ",") {
		searchedHeader = strings.TrimSpace(searchedHeader)
		if !reflect.DeepEqual(previousEntry.Request.Header[searchedHeader], r.Header[searchedHeader]) {
			return false
		}
	}

	return true
}
