ARG BASE_ALPINE
ARG BASE_GOLANG_17_ALPINE
FROM $BASE_GOLANG_17_ALPINE as artifact
RUN apk add --no-cache make git patch

RUN mkdir -p /src/kube-state-metrics && \
  wget https://github.com/kubernetes/kube-state-metrics/archive/v2.5.0.tar.gz -O - | tar -xz --strip-components=1 -C /src/kube-state-metrics
WORKDIR /src/kube-state-metrics
COPY patches/kube-persistentvolume-local.patch /
RUN patch -p1 < /kube-persistentvolume-local.patch
RUN make build-local

FROM $BASE_ALPINE
COPY --from=artifact /src/kube-state-metrics/kube-state-metrics /bin/kube-state-metrics
ENTRYPOINT ["/bin/kube-state-metrics"]
