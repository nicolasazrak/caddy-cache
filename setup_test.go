package cache

import (
	"time"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/mholt/caddy"
	"strconv"
)

func TestParsingConfig(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expect    Config
	}{
		{"cache /static", false, Config{
			CacheablePaths: []string{"/static"},
			RedisURL: "",
			DefaultMaxAge: time.Second * time.Duration(DEFAULTMAXAGE),
		}},
		{"cache /static {\n }", false, Config{
			CacheablePaths:[]string{"/static"},
			RedisURL: "",
			DefaultMaxAge: time.Second * time.Duration(DEFAULTMAXAGE),
		}},
		{"cache /api {\n path /assets \n}", false, Config{
			CacheablePaths:[]string{"/api", "/assets"},
			RedisURL: "",
			DefaultMaxAge: time.Second * time.Duration(DEFAULTMAXAGE),
		}},
		{"cache /static {\n default_max_age 30 \n}", false, Config{
			CacheablePaths:[]string{"/static"},
			RedisURL: "",
			DefaultMaxAge: time.Second * time.Duration(30),
		}},
		{"cache /api {\n redis redis://some.path/ \n}", false, Config{
			CacheablePaths:[]string{"/api"},
			RedisURL: "redis://some.path/",
			DefaultMaxAge: time.Second * time.Duration(DEFAULTMAXAGE),
		}},
		{"cache /api {\n redis redis://some.path/ \n default_max_age 30 \n path /public \n}", false, Config{
			CacheablePaths:[]string{"/api", "/public"},
			RedisURL: "redis://some.path/",
			DefaultMaxAge: time.Second * time.Duration(30),
		}},
		{"cache", true, Config{}}, // Missing default path
		{"cache /static {\n default_max_age anumber \n}", true, Config{}}, // max_age with invalid number
		{"cache /static {\n default_max_age 45 morepareters \n}", true, Config{}}, // More parameters
		{"cache /static {\n path \n}", true, Config{}}, // Missing parameters
		{"cache /static {\n max_age 50 \n}", true, Config{}}, // Unknown parameters
		{"cache /static {\n redis redis://some.path/ \n max_age 50 \n}", true, Config{}}, // Mixed valid and invalid parameters
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
