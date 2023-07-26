ARG BASE_GOLANG_17_ALPINE
ARG BASE_DISTROLESS
FROM $BASE_GOLANG_17_ALPINE as artifact
RUN apk add --no-cache make git patch

# Build KSM from sources in case of future patching
RUN mkdir -p /src/kube-state-metrics && \
  git clone --depth 1 --branch v2.6.0 ${SOURCE_REPO}/kubernetes/kube-state-metrics/ /src/kube-state-metrics
WORKDIR /src/kube-state-metrics
RUN make build-local

FROM $BASE_DISTROLESS
COPY --from=artifact /src/kube-state-metrics/kube-state-metrics /bin/kube-state-metrics
ENTRYPOINT ["/bin/kube-state-metrics"]
