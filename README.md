# caddy cache

[![Build Status](https://travis-ci.org/nicolasazrak/caddy-cache.svg?branch=master)](https://travis-ci.org/nicolasazrak/caddy-cache)


This is a simple caching plugin for [caddy server](https://caddyserver.com/)

To use it you need to compile your own version of caddy with this plugin like [this doc](https://github.com/mholt/caddy/wiki/Writing-a-Plugin:-Directives). 
 
Example minimal usage in `Caddyfile`

```
caddy.test {
    proxy / yourserver:5000
    cache
}
```

This will store in cache responses that specifically have a `Cache-control`, `Expires` or `Last-Modified` header set.

For more advanced usages you can use the following parameters: 

- `default_max_age:` Sets the default max age for responses without a `Cache-control` or `Expires` header. (Default: 60 seconds)
- `status_header:` Sets a header to add to the response indicating the status. It will respond with: skip, miss or hit
- `match:` Sets rules to make responses cacheable, if any matches and the response is cacheable by https://tools.ietf.org/html/rfc7234 then it will be stored. Supported options are:
    - `path`: check if the request starts with this path
    - `header`: checks if the response contains a header with one of the specified values
- `storage`: There are two storage engines:
    - `̀mmap` It stores the files contents in a file in /tmp You can specify where to store the files. Keep in mind that it is not persistent. Every time the server is restarted the files will be created again.
    - `memory` It stores the files contents in a byte array in memory

```
caddy.test {
    proxy / yourserver:5000
    cache {
        match {
            path /assets
            header Content-Type image/jpg image/png
        }
        default_max_age 10
        status_header X-Cache-Status
        storage mmap /tmp/caddy-cache
    }
}
```


### Benchmarks

Benchmark files are in `benchmark` folder. The backend server in the proxy case is [http-server](https://www.npmjs.com/package/http-server). Tests were run on my Lenovo G480 with Intel i3 3220 and 8gb of ram.

Test were executed with: `ab -n 2000 -c 25 http://caddy.test:2015/file.txt`


| File Size            ||                     41kb         ||             |      608kb            ||             |   2.6M                ||   
| ---                  |       :----:  |    :---:  |  :---: |    ----       |   ----   |   ----   |  :----:        |   ---  |   ---  |
|                      | **Total time**    | **Average**   | **99%th**  |  **Total time**   |  **Average** | **99%th**    | **Total time**     |  **Average**  | **99%th**  |
| Proxy + gzip         | 2.758 seconds | 34.472 ms |   63ms | 4.573 seconds | 57.164ms |  105ms   | 11.417 seconds | 142.716ms | 220ms  |
| Root  + gzip         | 0.268 seconds | 3.346ms   |   8ms  | 0.775 seconds |  9.689ms |   23ms   |  2.458 seconds |  30.729ms |  50ms  |
| Proxy + gzip + cache | 0.240 seconds | 3.002ms   |   7ms  | 0.743 seconds |  9.292ms |   16ms   |  2.380 seconds |  29.753ms |  35ms  |




### Todo list

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