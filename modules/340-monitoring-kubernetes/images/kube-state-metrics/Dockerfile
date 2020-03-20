FROM golang:1.12.1-alpine3.9
RUN mkdir -p /src/kube-state-metrics && \
  wget https://github.com/kubernetes/kube-state-metrics/archive/v1.8.0.tar.gz -O - | tar -xz --strip-components=1 -C /src/kube-state-metrics
WORKDIR /src/kube-state-metrics
ADD kube-persistentvolume-is-local.patch .
ADD kube-pod-spec-host-network.patch .
ADD kube-pod-status-reason.patch .
RUN apk add --no-cache make git
RUN patch -p1 < kube-persistentvolume-is-local.patch && \
  patch -p1 < kube-pod-spec-host-network.patch && \
  patch -p1 < kube-pod-status-reason.patch
RUN make build-local

FROM alpine:3.9
COPY --from=0 /src/kube-state-metrics/kube-state-metrics /bin/kube-state-metrics
ENTRYPOINT ["/bin/kube-state-metrics"]
