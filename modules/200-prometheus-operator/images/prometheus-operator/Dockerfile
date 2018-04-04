FROM golang:1.9-alpine3.7
WORKDIR /go/src/github.com/coreos/prometheus-operator
RUN wget https://github.com/coreos/prometheus-operator/archive/v0.17.0.tar.gz -O - | tar -xz --strip-components=1 && \
    apk --update add make git
ADD 923.patch .
ADD 1171.patch .
ADD 1172.patch .
RUN patch -p1 < 923.patch && \
    patch -p1 < 1171.patch && \
    patch -p1 < 1172.patch && \
    make build

FROM alpine:3.7
COPY --from=0 /go/src/github.com/coreos/prometheus-operator/operator /bin/
ENTRYPOINT [ "/bin/operator" ]
