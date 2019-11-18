FROM golang:1.12-alpine3.9
RUN apk add --no-cache git ca-certificates gcc build-base sqlite patch make
WORKDIR /dex
COPY crowd.patch saml-groups.patch client-groups.patch obsolete.patch static-user-groups.patch /
RUN wget https://github.com/dexidp/dex/archive/v2.18.0.tar.gz -O - | tar -xz --strip-components=1 \
  && patch -p1 < /crowd.patch \
  && patch -p1 < /saml-groups.patch \
  && patch -p1 < /client-groups.patch \
  && patch -p1 < /obsolete.patch \
  && patch -p1 < /static-user-groups.patch
RUN go build ./cmd/dex

FROM quay.io/dexidp/dex:v2.18.0
COPY --from=0 /dex/dex /usr/local/bin
COPY --from=0 /dex/web /web
