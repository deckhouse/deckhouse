ARG BASE_GOLANG_20_ALPINE_DEV
ARG BASE_DISTROLESS
FROM $BASE_GOLANG_20_ALPINE_DEV as artifact
ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Build KSM from sources in case of future patching
RUN mkdir -p /src/kube-state-metrics && \
  git clone --depth 1 --branch v2.7.0 ${SOURCE_REPO}/kubernetes/kube-state-metrics/ /src/kube-state-metrics
WORKDIR /src/kube-state-metrics
RUN go mod edit -go 1.20 \
    && go get golang.org/x/net@v0.17.0 \
    && go get github.com/emicklei/go-restful@v2.16.0+incompatible \
    && go mod tidy \
    && make build-local && \
    chown -R 64535:64535 /src/kube-state-metrics && \
    chmod 0700 /src/kube-state-metrics/kube-state-metrics

FROM $BASE_DISTROLESS
COPY --from=artifact /src/kube-state-metrics/kube-state-metrics /bin/kube-state-metrics

ENTRYPOINT ["/bin/kube-state-metrics"]
