package cache

import (
	"context"
	"crypto/tls"
	"net/http"
	"strconv"
	"testing"

	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/stretchr/testify/require"
)

func TestCacheKeyTemplating(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"{scheme} {host}{uri}", "https example.com/path?with=query"},
		{"{scheme} {host}", "https example.com"},
		{"{scheme} {uri}", "https /path?with=query"},
		{"{scheme}{uri}", "https/path?with=query"},
		{"{uri}", "/path?with=query"},
		{"{user}:{uri}", "bob:/path?with=query"},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r, err := http.NewRequest("GET", "https://bob:password@example.com/path?with=query", nil)
			if err != nil {
				require.NoError(t, err)
			}

			// Create a new replacer, that will be placed onto the context.
			replacer := httpserver.NewReplacer(r, nil, "")

			// Add in some custom fields.
			replacer.Set("user", "bob")

			// Swap out the context and inject the original url into the request.
			ctx := context.WithValue(r.Context(), httpserver.ReplacerCtxKey, replacer)
			ctx = context.WithValue(ctx, httpserver.OriginalURLCtxKey, *r.URL)
			r = r.WithContext(ctx)

			// As the input request contained https, we need to set the req.TLS to something to
			// match the templating behavior of the httpserver.Replacer.
			r.TLS = &tls.ConnectionState{}

			actual := getKey(test.input, r)
			require.Equal(t, test.expect, actual, "Invalid cache key computed in test "+strconv.Itoa(i+1))
		})
	}
}
