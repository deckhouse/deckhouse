ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE
WORKDIR /src/
RUN apk add --no-cache git mercurial patch make && \
    wget https://github.com/flant/cloud-provider-vsphere/archive/deckhouse-0.2.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
    make build

FROM $BASE_ALPINE
RUN apk add --no-cache ca-certificates
COPY --from=0 /src/.build/bin/vsphere-cloud-controller-manager.linux_amd64 /bin/vsphere-cloud-controller-manager
ENTRYPOINT [ "/bin/vsphere-cloud-controller-manager" ]
