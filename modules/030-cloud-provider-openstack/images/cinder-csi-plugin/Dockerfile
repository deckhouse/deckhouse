FROM golang:1.13-alpine3.11
WORKDIR /src/
RUN apk add --no-cache git mercurial patch make && \
    wget https://github.com/kubernetes/cloud-provider-openstack/archive/c99011b2538125194bbc77067584b27b05d482c2.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
    make cinder-csi-plugin

FROM alpine:3.11
COPY --from=0 /src/cinder-csi-plugin /bin/cinder-csi-plugin
ENTRYPOINT [ "/bin/cinder-csi-plugin" ]
