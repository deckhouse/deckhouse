FROM golang:1.14-alpine
WORKDIR /go/src/github.com/gardener/autoscaler
RUN apk add --no-cache linux-headers build-base git mercurial && \
    wget https://github.com/gardener/autoscaler/archive/v0.12.0.tar.gz -O - | tar -xz --strip-components=1 -C /go/src/github.com/gardener/autoscaler/ && \
    cd cluster-autoscaler && \
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o cluster-autoscaler

FROM alpine:3.11
COPY --from=0 /go/src/github.com/gardener/autoscaler/cluster-autoscaler/cluster-autoscaler /
ENTRYPOINT [ "/cluster-autoscaler" ]
