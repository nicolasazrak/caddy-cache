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

func TestWebSocketDetection(t *testing.T) {
	// Server receives a request with header line:
	//
	//	Connection: Websocket
	//
	chrome := http.Header{
		"Connection": {"Websocket"},
		"Upgrade":    {"Websocket"},
	}

	// Server receives a request with header line:
	//
	//	Connection: keep-alive, Websocket
	//
	firefox := http.Header{
		"Connection": {"keep-alive, Websocket"},
		"Upgrade":    {"Websocket"},
	}

	// Server receives a request with header lines:
	//
	//	Connection: keep-alive
	//	Connection: Websocket
	//
	other := http.Header{
		"Connection": {"keep-alive", "Websocket"},
		"Upgrade":    {"Websocket"},
	}

	safari := chrome

	// Server receives a request with header line:
	//
	//	Connection: notWebSocket
	//
	wrongConnection := http.Header{
		"Connection": {"notWebSocket"},
		"Upgrade":    {"Websocket"},
	}

	// Server receives a request with header line:
	//
	//	Upgrade: notWebSocket
	//
	wrongUpgrade := http.Header{
		"Connection": {"Websocket"},
		"Upgrade":    {"notWebsocket"},
	}

	// Websockets.
	require.Equal(t, isWebSocket(chrome), true, "Bad detection of Chrome headers")
	require.Equal(t, isWebSocket(firefox), true, "Bad detection of Firefox headers")
	require.Equal(t, isWebSocket(other), true, "Bad detection of other headers")
	require.Equal(t, isWebSocket(safari), true, "Bad detection of Safari headers")

	// Not websockets.
	require.Equal(t, isWebSocket(wrongConnection), false, "Bad detection of Connection header")
	require.Equal(t, isWebSocket(wrongUpgrade), false, "Bad detection of Upgrade header")
}
