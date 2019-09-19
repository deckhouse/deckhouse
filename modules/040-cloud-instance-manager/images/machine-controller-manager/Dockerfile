FROM golang:1.12-alpine
WORKDIR /src/
RUN apk add --no-cache git mercurial make bash
RUN wget https://github.com/flant/machine-controller-manager/archive/c02256bc9616f7f285edf2022736d242aed63f87.tar.gz -O - | tar -xz --strip-components=1 -C /src/
RUN make build-local

FROM alpine:3.10

RUN apk add --no-cache bash curl

# Resources used by Golang libary
ENV ZONEINFO=/zone-info/zoneinfo.zip
COPY --from=0 /src/assets/zoneinfo.zip /zone-info/zoneinfo.zip

COPY --from=0 /src/bin/machine-controller-manager /machine-controller-manager
WORKDIR /
ENTRYPOINT ["/machine-controller-manager"]
