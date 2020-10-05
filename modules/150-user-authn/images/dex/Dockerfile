FROM golang:1.13-alpine3.10
RUN apk add --no-cache git ca-certificates gcc build-base sqlite patch make
WORKDIR /dex
COPY bitbucket-groups.patch bitbucket-teams.patch client-groups.patch obsolete.patch static-user-groups.patch fix-relative-url.patch oidc-icon.patch /
RUN wget https://github.com/dexidp/dex/archive/v2.23.0.tar.gz -O - | tar -xz --strip-components=1 \
  && patch -p1 < /bitbucket-groups.patch \
  && patch -p1 < /bitbucket-teams.patch \
  && patch -p1 < /client-groups.patch \
  && patch -p1 < /obsolete.patch \
  && patch -p1 < /static-user-groups.patch \
  && patch -p1 < /fix-relative-url.patch \
  && patch -p1 < /oidc-icon.patch
RUN go build ./cmd/dex

FROM quay.io/dexidp/dex:v2.23.0
COPY --from=0 /dex/dex /usr/local/bin
COPY --from=0 /dex/web /web
