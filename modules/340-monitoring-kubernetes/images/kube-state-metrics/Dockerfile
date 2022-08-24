ARG BASE_ALPINE
ARG BASE_GOLANG_17_ALPINE
FROM $BASE_GOLANG_17_ALPINE as artifact
RUN apk add --no-cache make git patch

# Build KSM from sources in case of future patching
RUN mkdir -p /src/kube-state-metrics && \
  wget https://github.com/kubernetes/kube-state-metrics/archive/v2.6.0.tar.gz -O - | tar -xz --strip-components=1 -C /src/kube-state-metrics
WORKDIR /src/kube-state-metrics
RUN make build-local

FROM $BASE_ALPINE
COPY --from=artifact /src/kube-state-metrics/kube-state-metrics /bin/kube-state-metrics
ENTRYPOINT ["/bin/kube-state-metrics"]
