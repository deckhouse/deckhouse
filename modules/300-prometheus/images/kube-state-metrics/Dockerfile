FROM golang:1.12.1-alpine3.9
RUN mkdir -p /src/kube-state-metrics && \
  wget https://github.com/kubernetes/kube-state-metrics/archive/v1.6.0-rc.0.tar.gz -O - | tar -xz --strip-components=1 -C /src/kube-state-metrics
WORKDIR /src/kube-state-metrics
RUN apk add --no-cache make git
RUN make build-local

FROM alpine:3.9
COPY --from=0 /src/kube-state-metrics/kube-state-metrics /bin/kube-state-metrics
ENTRYPOINT ["/bin/kube-state-metrics"]

