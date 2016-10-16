# caddy cache

[![Build Status](https://travis-ci.org/nicolasazrak/caddy-cache.svg?branch=master)](https://travis-ci.org/nicolasazrak/caddy-cache)


This is a simple caching plugin for [caddy server](https://caddyserver.com/) backed by redis or an in memory store.

To use it you need to compile your own version of caddy with this plugin like [this doc](https://github.com/mholt/caddy/wiki/Writing-a-Plugin:-Directives). 
 
Example minimal usage in `Caddyfile`

```
caddy.test {
    proxy / yourserver:5000
    cache /static
}
```

First option is the base path to cache. Only files with path starting with /static will be cached.

For more advanced usages you can use the following parameters: 

- `path`: For adding more paths other than main path
- `default_max_age`: You can set the default max age for responses without a `Cache-control` or `Expires` header. (Default: 60 seconds)
- `redis`: If you want to use redis for caching just add the redis uri. (Note: this is not recommended if responses are large)

```
caddy.test {
    proxy / yourserver:5000
    cache /static {
        default_max_age 10
        redis redis://localhost:6379
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

- [ ] Support `vary` header
- [ ] Serve stale content if proxy is down
- [ ] Punch hole cache
- [ ] File disk storage for larger objects
- [ ] Add header with cache status
- [ ] Do conditional requests to revalidate data
- [ ] Max entries size