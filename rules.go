package cache

import (
	"net/http"
	"time"

	"github.com/pquerna/cachecontrol/cacheobject"
)

func getCacheableStatus(req *http.Request, statusCode int, respHeaders http.Header) (bool, time.Time, error) {
	reasonsNotToCache, expiration, err := cacheobject.UsingRequestResponse(req, statusCode, respHeaders, false)

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
