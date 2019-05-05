# caddy cache

[![Build Status](https://travis-ci.org/nicolasazrak/caddy-cache.svg?branch=master)](https://travis-ci.org/nicolasazrak/caddy-cache)

This is a simple caching plugin for [caddy server](https://caddyserver.com/)

**Notice:** Although this plugin works with static content it is not advised. Static content will not see great benefits. It should be used when there are slow responses, for example when using caddy as a proxy to a slow backend.

## Build

**Notice:** Build requires Go 1.12 or higher.

To use it you need to compile your own version of caddy with this plugin. First fetch the code

- `export GO111MODULE=on`
- `go get -u github.com/mholt/caddy/...`
- `go get -u github.com/nicolasazrak/caddy-cache/...`

Then update the file in `$GOPATH/src/github.com/mholt/caddy/caddy/caddymain/run.go` and import `_ "github.com/nicolasazrak/caddy-cache"`.

And finally build caddy with:

- `cd $GOPATH/src/github.com/mholt/caddy/caddy`
- `./build.bash`

This will produce the caddy binary in that same folder. For more information about how plugins work read [this doc](https://github.com/mholt/caddy/wiki/Writing-a-Plugin:-Directives). 

## Usage

Example minimal usage in `Caddyfile`

```
caddy.test {
    proxy / yourserver:5000
    cache
}
```

This will store in cache responses that specifically have a `Cache-control`, `Expires` or `Last-Modified` header set.

For more advanced usages you can use the following parameters: 

- `match_path`: Paths to cache. For example `match_path /assets` will cache all successful responses for requests that start with /assets and are not marked as private.
- `match_header`: Matches responses that have the selected headers. For example `match_header Content-Type image/png image/jpg` will cache all successful responses that with content type `image/png` OR `image/jpg`. Note that if more than one is specified, anyone that matches will make the response cacheable. 
- `path`: Path where to store the cached responses. By default it will use the operating system temp folder.
- `default_max_age`: Max-age to use for matched responses that do not have an explicit expiration. (Default: 5 minutes)
- `status_header`: Sets a header to add to the response indicating the status. It will respond with: skip, miss or hit. (Default: `X-Cache-Status`)
- `cache_key`: Configures the cache key using [Placeholders]
(https://caddyserver.com/docs/placeholders), it supports any of the request placeholders. (Default: `{method} {host}{path}?{query}`)
- `header_upstream` sets headers to be passed to the source address. The field name is name and the value is value. This option can be specified multiple times for multiple headers, and dynamic values can also be inserted using [request placeholders](https://caddyserver.com/docs/placeholders). By default, existing header fields will be replaced, but you can add/merge field values by prefixing the field name with a plus sign (+). You can remove fields by prefixing the header name with a minus sign (-) and leaving the value blank.
- `header_downstream` modifies response headers coming back from the source address. It works the same way header_upstream does.

```
caddy.test {
    proxy / yourserver:5000
    cache {
        match_path /assets
        header_upstream X-Request-By cross-CDN
        header_downstream -Server
        match_header Content-Type image/jpg image/png
        status_header X-Cache-Status
        default_max_age 15m
        path /tmp/caddy-cache
    }
}
```


### Logs

Caddy-cache adds a `{cache_status}` placeholder that can be used in logs.

## Benchmarks

Benchmark files are in `benchmark` folder. Tests were run on my Lenovo G480 with Intel i3 3220 and 8gb of ram.

- First test: Download `sherlock.txt` (608 Kb) file from the root (caddy.test3), a proxy to root server (caddy.test2) and a proxy to root server with cache (caddy.test).

    `wrk -c 400 -d 30s --latency -t 4 http://caddy.test:2015/sherlock.txt`

    |               | Req/s   | Throughput  | 99th Latency |
    |---------------|---------|-------------|--------------|
    | proxy + cache | 4548.03 |   2.64 GB/s | 561.39 ms    |
    | proxy         | 1043.61 | 619.65 MB/s |   1.00 s     |
    | root          | 3668.14 |   2.13 GB/s | 612.39 ms    |

- Second test: Download `montecristo.txt` (2,6 Mb) file from the root (caddy.test3), a proxy to root server (caddy.test2) and a proxy to root server with cache (caddy.test).

    `wrk -c 400 -d 30s --latency -t 4 http://caddy.test:2015/montecristo.txt`

    |               | Req/s   | Throughput  | 99th Latency |
    |---------------|---------|-------------|--------------|
    | proxy + cache | 1199.81 |   3.01 GB/s | 1.65 s       |
    | proxy         |  473.14 |   1.20 GB/s | 1.81 s       |
    | root          | 1064.44 |   2.66 GB/s | 1.71 s       |

- Third test: Download `pg31674.txt` (41 Kb) a root server (caddy.test5) with gzip and a proxy to root server with cache (caddy.test4).

    `wrk -c 50 -d 30s --latency -H 'Accept-Encoding: gzip' -t 4 http://caddy.test4:2015/pg31674.txt`

    |               | Req/s    | Throughput  | 99th Latency |
    |---------------|----------|-------------|--------------|
    | proxy + cache | 16547.84 | 242.05 MB/s |  22.48ms     |
    | root          | 792.08   |  11.60 MB/s | 109.98ms     |

## Todo list

- [x] Support `vary` header
- [x] Add header with cache status
- [x] Stream responses while fetching them from upstream
- [x] Locking concurrent requests to the same path
- [x] File disk storage for larger objects
- [x] Add a configuration to not use query params in cache key (via `cache_key` directive)
- [ ] Purge cache entries [#1](https://github.com/nicolasazrak/caddy-cache/issues/1)
- [ ] Serve stale content if proxy is down
- [ ] Punch hole cache
- [ ] Do conditional requests to revalidate data
- [ ] Max entries size
