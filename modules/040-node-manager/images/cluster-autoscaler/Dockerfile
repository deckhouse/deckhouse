FROM golang:1.14-alpine
WORKDIR /go/src/github.com/gardener/autoscaler
COPY apiv1_daemonset.patch /
COPY "do_not_violate_NodeGroup's_MinSize_when_fixing_NodeGroup.patch" /
RUN apk add --no-cache linux-headers build-base git mercurial patch && \
    wget https://github.com/gardener/autoscaler/archive/0.9.0.tar.gz -O - | tar -xz --strip-components=1 -C /go/src/github.com/gardener/autoscaler/ && \
    patch -p1 < /apiv1_daemonset.patch && \
    patch -p0 < "/do_not_violate_NodeGroup's_MinSize_when_fixing_NodeGroup.patch" && \
    cd cluster-autoscaler && \
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o cluster-autoscaler

FROM alpine:3.11
COPY --from=0 /go/src/github.com/gardener/autoscaler/cluster-autoscaler/cluster-autoscaler /
ENTRYPOINT [ "/cluster-autoscaler" ]
