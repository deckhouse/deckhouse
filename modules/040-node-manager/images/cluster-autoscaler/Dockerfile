ARG BASE_ALPINE
ARG  BASE_GOLANG_18_ALPINE
FROM $BASE_GOLANG_18_ALPINE as artifact
WORKDIR /go/src/github.com/gardener/autoscaler
COPY "patches/daemonset-eviction.patch" /
COPY "patches/scale-from-zero.patch" /

RUN apk add --no-cache linux-headers build-base git mercurial patch && \
    wget https://github.com/gardener/autoscaler/archive/v1.24.0.tar.gz -O - | tar -xz --strip-components=1 -C /go/src/github.com/gardener/autoscaler/ && \
    patch -p1 < "/daemonset-eviction.patch" && \
    patch -p1 < "/scale-from-zero.patch" && \
    cd cluster-autoscaler && \
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o cluster-autoscaler

FROM $BASE_ALPINE
COPY --from=artifact /go/src/github.com/gardener/autoscaler/cluster-autoscaler/cluster-autoscaler /
ENTRYPOINT [ "/cluster-autoscaler" ]
