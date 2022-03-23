ARG BASE_DEBIAN
FROM registry.deckhouse.io/base_images/rust:1.57.0@sha256:7d2592d1a6a73055fbadc0c76868046736ce5fa834ed5c201e3b21f27c28947e as build

RUN apt-get update \
    && apt-get install -yq \
      ca-certificates make bash cmake libclang1-9 llvm-9 libsasl2-dev librdkafka-dev

WORKDIR /vector
COPY patches/loki-labels.patch patches/kubernetes-logs-lib.patch /
RUN git clone --depth 1 --branch v0.20.0 https://github.com/timberio/vector.git
    # && git apply /loki-labels.patch \
    # && git apply /kubernetes-logs-lib.patch

# Download and cache dependencies
WORKDIR /vector/vector
RUN cargo fetch

RUN cargo build \
    --release \
    -j $(($(nproc) /2)) \
    --offline \
    --no-default-features \
    --features "api,api-client,enrichment-tables,sources-internal_metrics,sources-file,sources-kubernetes_logs,transforms,sinks-prometheus,sinks-blackhole,sinks-elasticsearch,sinks-file,sinks-loki,sinks-socket,unix,vendor-all,vrl-cli" \
    && strip target/release/vector

FROM debian:bullseye
COPY --from=build /vector/vector/target/release/vector /usr/bin/vector
RUN mkdir -p /etc/vector \
    && apt-get update \
    && apt-get install -yq ca-certificates tzdata \
    && rm -rf /var/cache/apt/archives/*
ENTRYPOINT ["/usr/bin/vector"]
