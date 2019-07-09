FROM golang:alpine
LABEL caddy_version="dev" architecture="amd64"

ENV GOPATH /go

RUN    apk -U --no-progress upgrade \
    && apk update \
    && apk add --upgrade busybox sed bash \
    && apk -U --force --no-progress add git \
    && git clone https://github.com/caddyserver/caddy.git /go/src/github.com/caddyserver/caddy \
    && cd /go/src/github.com/caddyserver/caddy \
    && git config --global user.email "caddy@caddyserver.com" \
    && git config --global user.name "caddy" \
    && go get ./... \
    && go get -u github.com/nicolasazrak/caddy-cache \
    && cd /go/src/github.com/nicolasazrak/caddy-cache \
    && go get ./... \
    && sed -i -z 's/"io\/ioutil"/"io\/ioutil"\n\t_ "github.com\/nicolasazrak\/caddy-cache"/' /go/src/github.com/caddyserver/caddy/caddy/caddymain/run.go

RUN cd /go/src/github.com/caddyserver/caddy/caddy && ./build.bash "/usr/bin/caddy" \
    #&& mv /go/bin/caddy /usr/bin \
    && apk del --purge git sed bash \
    && rm -rf $GOPATH /var/cache/apk/*

WORKDIR /srv

EXPOSE 80 443 2015
VOLUME     ["/root/.caddy"]
ENTRYPOINT ["/usr/bin/caddy"]
CMD ["--conf", "/etc/Caddyfile", "--log", "stdout"]
