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

func matchesVary(currentRequest *http.Request, previousResponse *Response) bool {
	vary := previousResponse.HeaderMap.Get("Vary")

	for _, searchedHeader := range strings.Split(vary, ",") {
		searchedHeader = strings.TrimSpace(searchedHeader)
		if !reflect.DeepEqual(currentRequest.Header[searchedHeader], previousResponse.HeaderMap[searchedHeader]) {
			return false
		}
	}

	return true
}
