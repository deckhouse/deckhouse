FROM golang:1.12-alpine
WORKDIR /go/src/github.com/gardener/autoscaler
COPY mcm.patch /
RUN apk add --no-cache linux-headers build-base git mercurial patch && \
    wget https://github.com/gardener/autoscaler/archive/0.5.0.tar.gz -O - | tar -xz --strip-components=1 -C /go/src/github.com/gardener/autoscaler/ && \
    patch -p1 < /mcm.patch && \
    cd cluster-autoscaler && \
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o cluster-autoscaler

FROM alpine:3.10
COPY --from=0 /go/src/github.com/gardener/autoscaler/cluster-autoscaler/cluster-autoscaler /
ENTRYPOINT [ "/cluster-autoscaler" ]
