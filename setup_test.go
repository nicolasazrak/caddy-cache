package cache

import (
	"strconv"
	"testing"
	"time"

	"github.com/caddyserver/caddy"
	"github.com/stretchr/testify/require"
)

func TestParsingConfig(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expect    Config
	}{
		{"cache", false, Config{
			StatusHeader:     defaultStatusHeader,
			LockTimeout:      defaultLockTimeout,
			DefaultMaxAge:    defaultMaxAge,
			CacheRules:       []CacheRule{},
			CacheKeyTemplate: defaultCacheKeyTemplate,
		}},
		{"cache {\n match_path /assets \n} }", false, Config{
			StatusHeader:     defaultStatusHeader,
			LockTimeout:      defaultLockTimeout,
			DefaultMaxAge:    defaultMaxAge,
			CacheRules:       []CacheRule{&PathCacheRule{Path: "/assets"}},
			CacheKeyTemplate: defaultCacheKeyTemplate,
		}},
		{"cache {\n match_path /assets \n match_path /api \n} \n}", false, Config{
			StatusHeader:  defaultStatusHeader,
			LockTimeout:   defaultLockTimeout,
			DefaultMaxAge: defaultMaxAge,
			CacheRules: []CacheRule{
				&PathCacheRule{Path: "/assets"},
				&PathCacheRule{Path: "/api"},
			},
			CacheKeyTemplate: defaultCacheKeyTemplate,
		}},
		{"cache {\n match_header Content-Type image/png image/gif \n match_path /assets \n}", false, Config{
			StatusHeader:  defaultStatusHeader,
			LockTimeout:   defaultLockTimeout,
			DefaultMaxAge: defaultMaxAge,
			CacheRules: []CacheRule{
				&HeaderCacheRule{Header: "Content-Type", Value: []string{"image/png", "image/gif"}},
				&PathCacheRule{Path: "/assets"},
			},
			CacheKeyTemplate: defaultCacheKeyTemplate,
		}},
		{"cache {\n status_header X-Custom-Header \n}", false, Config{
			StatusHeader:     "X-Custom-Header",
			LockTimeout:      defaultLockTimeout,
			DefaultMaxAge:    defaultMaxAge,
			CacheRules:       []CacheRule{},
			CacheKeyTemplate: defaultCacheKeyTemplate,
		}},
		{"cache {\n path /tmp/caddy \n}", false, Config{
			StatusHeader:     defaultStatusHeader,
			LockTimeout:      defaultLockTimeout,
			DefaultMaxAge:    defaultMaxAge,
			CacheRules:       []CacheRule{},
			Path:             "/tmp/caddy",
			CacheKeyTemplate: defaultCacheKeyTemplate,
		}},
		{"cache {\n lock_timeout 1s \n}", false, Config{
			StatusHeader:     defaultStatusHeader,
			LockTimeout:      time.Duration(1) * time.Second,
			DefaultMaxAge:    defaultMaxAge,
			CacheRules:       []CacheRule{},
			CacheKeyTemplate: defaultCacheKeyTemplate,
		}},
		{"cache {\n default_max_age 1h \n}", false, Config{
			StatusHeader:     defaultStatusHeader,
			LockTimeout:      defaultLockTimeout,
			DefaultMaxAge:    time.Duration(1) * time.Hour,
			CacheRules:       []CacheRule{},
			CacheKeyTemplate: defaultCacheKeyTemplate,
		}},
		{"cache {\n cache_key \"{scheme} {host}{uri}\" \n}", false, Config{
			StatusHeader:     defaultStatusHeader,
			LockTimeout:      defaultLockTimeout,
			DefaultMaxAge:    defaultMaxAge,
			CacheRules:       []CacheRule{},
			CacheKeyTemplate: "{scheme} {host}{uri}",
		}},
		{"cache {\n match_header aheader \n}", true, Config{}},          // match_header without value
		{"cache {\n lock_timeout aheader \n}", true, Config{}},          // lock_timeout with invalid duration
		{"cache {\n lock_timeout \n}", true, Config{}},                  // lock_timeout has no arguments
		{"cache {\n default_max_age somevalue \n}", true, Config{}},     // lock_timeout has invalid duration
		{"cache {\n default_max_age \n}", true, Config{}},               // default_max_age has no arguments
		{"cache {\n status_header aheader another \n}", true, Config{}}, // status_header with invalid number of parameters
		{"cache {\n match_path / ea \n}", true, Config{}},               // Invalid number of parameters in match
		{"cache {\n invalid / ea \n}", true, Config{}},                  // Invalid directive
		{"cache {\n path \n}", true, Config{}},                          // Path without arguments
		{"cache {\n cache_key \n}", true, Config{}},                     // cache_key without arguments
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			c := caddy.NewTestController("http", test.input)
			actual, err := cacheParse(c)

			if test.shouldErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expect, *actual, "Invalid config parsed in test "+strconv.Itoa(i+1))
			}
		})
	}

}
