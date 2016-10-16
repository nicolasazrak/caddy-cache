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

### Todo list

- [ ] Support `vary` header
- [ ] Serve stale content if proxy is down
- [ ] Punch hole cache
- [ ] File disk storage for larger objects
- [ ] Add header with cache status
- [ ] Do conditional requests to revalidate data
- [ ] Max entries size