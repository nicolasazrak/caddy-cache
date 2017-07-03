package cache

import (
	"testing"
)

func TestParsingConfig(t *testing.T) {
	// cacheAssetsRule := PathCacheRule{
	// 	Path: "/assets",
	// }

	tests := []struct {
		input     string
		shouldErr bool
		expect    Config
	}{
		{"cache", false, Config{
		// Storage:       NewMMapStorage("/tmp/caddy-cache"),
		// CacheRules:    []CacheRule{},
		// DefaultMaxAge: DEFAULT_MAX_AGE,
		}},
		{"cache {\n match path /assets \n} }", false, Config{
		// Storage:       NewMMapStorage("/tmp/caddy-cache"),
		// CacheRules:    []CacheRule{&cacheAssetsRule},
		// DefaultMaxAge: DEFAULT_MAX_AGE,
		}},
		{"cache {\n match path /assets \n match path /api \n} \n}", false, Config{
		// Storage: NewMMapStorage("/tmp/caddy-cache"),
		// CacheRules: []CacheRule{
		// 	&cacheAssetsRule,
		// 	&PathCacheRule{Path: "/api"},
		// },
		// DefaultMaxAge: DEFAULT_MAX_AGE,
		}},
		{"cache {\n match path /assets \n default_max_age 30 \n}", false, Config{
		// Storage:       NewMMapStorage("/tmp/caddy-cache"),
		// CacheRules:    []CacheRule{&cacheAssetsRule},
		// DefaultMaxAge: time.Second * time.Duration(30),
		}},
		{"cache {\n default_max_age 30 \n match path /public \n}", false, Config{
		// Storage:       NewMMapStorage("/tmp/caddy-cache"),
		// CacheRules:    []CacheRule{&PathCacheRule{Path: "/public"}},
		// DefaultMaxAge: time.Second * time.Duration(30),
		}},
		{"cache {\n match header Content-Type image/png image/gif \n match path /assets \n}", false, Config{
		// Storage: NewMMapStorage("/tmp/caddy-cache"),
		// CacheRules: []CacheRule{
		// 	&HeaderCacheRule{
		// 		Header: "Content-Type",
		// 		Value:  []string{"image/png", "image/gif"},
		// 	},
		// 	&cacheAssetsRule,
		// },
		// DefaultMaxAge: DEFAULT_MAX_AGE,
		}},
		{"cache {\n status_header X-Custom-Header \n}", false, Config{
		// Storage:       NewMMapStorage("/tmp/caddy-cache"),
		// CacheRules:    []CacheRule{},
		// StatusHeader:  "X-Custom-Header",
		// DefaultMaxAge: DEFAULT_MAX_AGE,
		}},
		{"cache {\n storage mmap /some/path \n}", false, Config{
		// Storage:       NewMMapStorage("/some/path"),
		// CacheRules:    []CacheRule{},
		// DefaultMaxAge: DEFAULT_MAX_AGE,
		}},
		{"cache {\n storage memory \n}", false, Config{
		// Storage:       NewMemoryStorage(),
		// CacheRules:    []CacheRule{},
		// DefaultMaxAge: DEFAULT_MAX_AGE,
		}},
		{"cache {\n status_header aheader another \n}", true, Config{}},    // status_header with invalid number of parameters
		{"cache {\n default_max_age anumber \n}", true, Config{}},          // max_age with invalid number
		{"cache {\n default_max_age 45 morepareters \n}", true, Config{}},  // More parameters
		{"cache {\n default_max_age \n}", true, Config{}},                  // Missing parameters
		{"cache {\n max_age 50 \n}", true, Config{}},                       // Unknown parameters
		{"cache {\n default_max_age 20 \n max_age 50 \n}", true, Config{}}, // Mixed valid and invalid parameters
		{"cache {\n match path / ea \n}", true, Config{}},                  // Invalid number of parameters in match
		{"cache {\n match unknown \n}", true, Config{}},                    // Unknown condition in match
		{"cache {\n match \n}", true, Config{}},                            // Unknown "invalid"
		{"cache {\n storage pepe \n}", true, Config{}},                     // Unknown storage "pepe"
	}

	for range /* i, test := */ tests {
		// t.Run(strconv.Itoa(i), func(t *testing.T) {
		// c := caddy.NewTestController("http", test.input)
		// actual, err := cacheParse(c)

		// if err != nil && !test.shouldErr {
		// 	t.Fatal(err)
		// }

		// if test.shouldErr {
		// 	assert.Error(t, err)
		// } else {
		// 	assert.NoError(t, err)
		// 	assert.Equal(t, test.expect, *actual, "Invalid config parsed in test "+strconv.Itoa(i+1))
		// }
		// })
	}

}
