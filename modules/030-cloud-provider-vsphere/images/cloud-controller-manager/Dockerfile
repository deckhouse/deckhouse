FROM golang:1.13-alpine3.11
WORKDIR /src/
RUN apk add --no-cache git mercurial patch make && \
    wget https://github.com/flant/cloud-provider-vsphere/archive/deckhouse-0.2.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
    make build

FROM alpine:3.11
RUN apk add --no-cache ca-certificates
COPY --from=0 /src/.build/bin/vsphere-cloud-controller-manager.linux_amd64 /bin/vsphere-cloud-controller-manager
ENTRYPOINT [ "/bin/vsphere-cloud-controller-manager" ]
