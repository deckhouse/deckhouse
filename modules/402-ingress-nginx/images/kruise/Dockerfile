ARG BASE_ALPINE
ARG BASE_GOLANG_19_BULLSEYE

FROM $BASE_GOLANG_19_BULLSEYE as builder
ARG KRUISE_CONTROLLER_VERSION=1.4.0

COPY patches/disable-controllers.patch /

RUN git clone --depth 1 --branch v${KRUISE_CONTROLLER_VERSION} https://github.com/openkruise/kruise.git && \
    cd kruise && \
    git apply /disable-controllers.patch && \
    make build && \
    mv bin/manager /tmp/manager


FROM $BASE_ALPINE
RUN apk add --no-cache ca-certificates bash expat && \
    rm -rf /var/cache/apk/*
WORKDIR /
COPY --from=builder /tmp/manager /manager
ENTRYPOINT ["/manager"]
