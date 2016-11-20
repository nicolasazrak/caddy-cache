package cache

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/mholt/caddy"
	"strconv"
	"time"
)


func TestParsingConfig(t *testing.T) {
	cacheAssetsRule := PathCacheRule {
		Path: "/assets",
	}

	tests := []struct {
		input     string
		shouldErr bool
		expect    Config
	}{
		{"cache", false, Config{
			CacheRules: []CacheRule{},
			RedisURL: "",
			DefaultMaxAge: DEFAULT_MAX_AGE,
		}},
		{"cache {\n match { \n path /assets \n} }", false, Config{
			CacheRules: []CacheRule{ &cacheAssetsRule },
			RedisURL: "",
			DefaultMaxAge: DEFAULT_MAX_AGE,
		}},
		{"cache {\n match { \n path /assets \n path /api  \n} \n}", false, Config{
			CacheRules:[]CacheRule{
				&cacheAssetsRule,
				&PathCacheRule{ Path: "/api" },
			},
			RedisURL: "",
			DefaultMaxAge: DEFAULT_MAX_AGE,
		}},
		{"cache {\n default_max_age 30 \n match { \n path /assets \n } \n}", false, Config{
			CacheRules: []CacheRule{ &cacheAssetsRule },
			RedisURL: "",
			DefaultMaxAge: time.Second * time.Duration(30),
		}},
		{"cache {\n redis redis://some.path/ \n match { \n path /assets \n } \n}", false, Config{
			CacheRules:[]CacheRule{ &cacheAssetsRule },
			RedisURL: "redis://some.path/",
			DefaultMaxAge: DEFAULT_MAX_AGE,
		}},
		{"cache {\n redis redis://some.path/ \n default_max_age 30 \n match { \n path /public \n } \n}", false, Config{
			CacheRules: []CacheRule{ &PathCacheRule{ Path: "/public" } },
			RedisURL: "redis://some.path/",
			DefaultMaxAge: time.Second * time.Duration(30),
		}},
		{"cache {\n match { header Content-Type image/png image/gif \n path /assets \n } \n}", false, Config{
			CacheRules: []CacheRule {
				&HeaderCacheRule {
					Header: "Content-Type",
					Value: []string { "image/png", "image/gif" },
				},
				&cacheAssetsRule,
			},
			DefaultMaxAge: DEFAULT_MAX_AGE,
		}},
		{"cache {\n default_max_age anumber \n}", true, Config{}}, // max_age with invalid number
		{"cache {\n default_max_age 45 morepareters \n}", true, Config{}}, // More parameters
		{"cache {\n default_max_age \n}", true, Config{}}, // Missing parameters
		{"cache {\n max_age 50 \n}", true, Config{}}, // Unknown parameters
		{"cache {\n redis redis://some.path/ \n max_age 50 \n}", true, Config{}}, // Mixed valid and invalid parameters
		{"cache {\n match { \n path / ea \n} \n}", true, Config{}}, // Invalid number of parameters in match
		{"cache {\n match { \n unknown \n} \n}", true, Config{}}, // Unknown condition in match
	}

	for i, test := range tests {
		c := caddy.NewTestController("http", test.input)
		actual, err := cacheParse(c)

		if err != nil && !test.shouldErr {
			t.Fatal(err)
		}

		if test.shouldErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, test.expect, *actual, "Invalid config parsed in test " + strconv.Itoa(i + 1))
		}
	}

}
