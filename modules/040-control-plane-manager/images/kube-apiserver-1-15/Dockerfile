FROM golang:1.12-alpine3.11
WORKDIR /src/
COPY old_certs.patch /
RUN apk add --no-cache make bash git mercurial patch rsync && \
    wget https://github.com/kubernetes/kubernetes/archive/v1.15.11.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
    patch -p0 < /old_certs.patch && \
    make all WHAT=cmd/kube-apiserver

FROM alpine:3.11
COPY --from=0 /src/_output/bin/kube-apiserver /usr/local/bin/kube-apiserver
COPY --from=k8s.gcr.io/pause:3.1 /pause /pause
ENTRYPOINT [ "/usr/local/bin/kube-apiserver" ]
