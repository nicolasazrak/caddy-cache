# caddy cache

[![Build Status](https://travis-ci.org/nicolasazrak/caddy-cache.svg?branch=master)](https://travis-ci.org/nicolasazrak/caddy-cache)


This is a simple caching plugin for [caddy server](https://caddyserver.com/) backed by redis.

To use it you need to compile your own version of caddy with this plugin like [this doc](https://github.com/mholt/caddy/wiki/Writing-a-Plugin:-Directives). 
 
Example usage in `Caddyfile`

```
caddy.test {
    proxy / yourserver:5000
    cache redis://localhost:6379
}
```


### Todo list

- [ ] Serve stale content if proxy is down
- [ ] Punch hole cache
- [ ] File disk storage for larger objects
- [ ] Add header with cache status


#### Wishlist
 
- [ ] Support vary header
- [ ] Do conditional requests to revalidate data
