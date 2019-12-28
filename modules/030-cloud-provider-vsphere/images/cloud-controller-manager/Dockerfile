FROM golang:1.12.11-alpine3.10
WORKDIR /src/
RUN apk add --no-cache git mercurial patch make && \
    wget https://github.com/flant/cloud-provider-vsphere/archive/7460f7d530363da7a1cfba09854eb3ad69cdfec1.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
    make build

FROM alpine:3.10
COPY --from=0 /src/.build/bin/vsphere-cloud-controller-manager.linux_amd64 /bin/vsphere-cloud-controller-manager
ENTRYPOINT [ "/bin/vsphere-cloud-controller-manager" ]
