FROM golang:1.11-alpine3.7
WORKDIR /go/src/github.com/coreos/prometheus-operator
RUN wget https://github.com/coreos/prometheus-operator/archive/v0.28.0.tar.gz -O - | tar -xz --strip-components=1 && \
    apk --update add bash make git
ADD disk-based-retention.patch .
RUN patch -p1 < disk-based-retention.patch && \
    make build

FROM alpine:3.7
COPY --from=0 /go/src/github.com/coreos/prometheus-operator/operator /bin/
ENTRYPOINT [ "/bin/operator" ]
