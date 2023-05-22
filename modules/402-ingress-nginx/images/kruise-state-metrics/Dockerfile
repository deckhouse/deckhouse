ARG BASE_ALPINE
ARG BASE_GOLANG_19_BULLSEYE

FROM $BASE_GOLANG_19_BULLSEYE as builder
ARG KRUISE_STATE_METRICS_BRANCH=master
ARG KRUISE_STATE_METRICS_COMMIT=b081f2ae9e011fd92fd23f6efa209601f5a20a01

RUN git clone --depth 1 --branch ${KRUISE_STATE_METRICS_BRANCH} https://github.com/openkruise/kruise-state-metrics.git && \
    cd kruise-state-metrics && git checkout ${KRUISE_STATE_METRICS_COMMIT} && \
    CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -a -o /tmp/kruise-state-metrics main.go

FROM $BASE_ALPINE
RUN apk add --no-cache ca-certificates bash expat && \
    rm -rf /var/cache/apk/*
WORKDIR /
COPY --from=builder /tmp/kruise-state-metrics /kruise-state-metrics
ENTRYPOINT ["/kruise-state-metrics"]
