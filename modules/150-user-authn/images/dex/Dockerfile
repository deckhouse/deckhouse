FROM golang:1.12-alpine3.9
RUN apk add --no-cache git ca-certificates gcc build-base sqlite patch
WORKDIR /dex
COPY crowd.patch /crowd.patch
RUN wget https://github.com/dexidp/dex/archive/v2.18.0.tar.gz -O - | tar -xz --strip-components=1 \
  && patch -p1 < /crowd.patch
RUN go build ./cmd/dex

FROM quay.io/dexidp/dex:v2.18.0
COPY --from=0 /dex/dex /usr/local/bin
