# caddy cache

[![Build Status](https://travis-ci.org/nicolasazrak/caddy-cache.svg?branch=master)](https://travis-ci.org/nicolasazrak/caddy-cache)

**Warning: This plugin is still experimental, it should not be used in production, it doesn't handle concurrent requests. There is work in progress branch to solve that https://github.com/nicolasazrak/caddy-cache/tree/v2 (more information: https://github.com/nicolasazrak/caddy-cache/issues/9#issuecomment-310868633)**

This is a simple caching plugin for [caddy server](https://caddyserver.com/)

## Build

To use it you need to compile your own version of caddy with this plugin. First fetch the code

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

```
caddy.test {
    proxy / yourserver:5000
    cache {
        match_path /assets
        match_header Content-Type image/jpg image/png
        status_header X-Cache-Status
        default_max_age 15m
        path /tmp/caddy-cache
    }
}
```


## Benchmarks

Benchmark files are in `benchmark` folder. Tests were run on my Lenovo G480 with Intel i3 3220 and 8gb of ram.

- First test: Download `sherlock.txt` (608 Kb) file from the root (caddy.test3), a proxy to root server (caddy.test2) and a proxy to root server with cache (caddy.test).

    `wrk -c 400 -d 30s --latency -t 4 http://caddy.test:2015/sherlock.txt`

    |               | Req/s   | Throughput  | 99th Latency |
    |---------------|---------|-------------|--------------|
    | proxy + cache | 4295.73 |   2.49 GB/s | 563.62 ms    |
    | proxy         | 1043.61 | 619.65 MB/s |   1.00 s     |
    | root          | 3668.14 |   2.13 GB/s | 612.39 ms    |

- Second test: Download `montecristo.txt` (2,6 Mb) file from the root (caddy.test3), a proxy to root server (caddy.test2) and a proxy to root server with cache (caddy.test).

    `wrk -c 400 -d 30s --latency -t 4 http://caddy.test:2015/montecristo.txt`

    |               | Req/s   | Throughput  | 99th Latency |
    |---------------|---------|-------------|--------------|
    | proxy + cache | 1220.69 |   3.07 GB/s | 1.69 s       |
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
- [ ] Purge cache entries [#1](https://github.com/nicolasazrak/caddy-cache/issues/1)
- [ ] Serve stale content if proxy is down
- [ ] Punch hole cache
- [ ] Do conditional requests to revalidate data
- [ ] Max entries size
- [ ] Add a configuration to not use query params in cache key