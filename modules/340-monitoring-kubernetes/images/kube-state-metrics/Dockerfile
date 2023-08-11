ARG BASE_GOLANG_17_ALPINE
ARG BASE_DISTROLESS
FROM $BASE_GOLANG_17_ALPINE as artifact
RUN apk add --no-cache make git patch

ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}

# Build KSM from sources in case of future patching
RUN mkdir -p /src/kube-state-metrics && \
  git clone --depth 1 --branch v2.6.0 ${SOURCE_REPO}/kubernetes/kube-state-metrics/ /src/kube-state-metrics
WORKDIR /src/kube-state-metrics
RUN make build-local && \
    chown -R 64535:64535 /src/kube-state-metrics && \
    chmod 0700 /src/kube-state-metrics/kube-state-metrics

FROM $BASE_DISTROLESS
COPY --from=artifact /src/kube-state-metrics/kube-state-metrics /bin/kube-state-metrics
USER 64535
ENTRYPOINT ["/bin/kube-state-metrics"]
