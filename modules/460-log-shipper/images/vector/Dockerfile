ARG BASE_DEBIAN_BULLSEYE
ARG BASE_RUST

FROM $BASE_RUST as build

RUN apt-get update \
    && apt-get install -yq \
      ca-certificates make bash cmake libclang1-9 llvm-9 libsasl2-dev librdkafka-dev

WORKDIR /vector
COPY patches/loki-labels.patch patches/kubernetes-owner-reference.patch /
RUN git clone --depth 1 --branch v0.20.0 https://github.com/vectordotdev/vector.git \
    && cd vector \
    && git apply /kubernetes-owner-reference.patch \
    && git apply /loki-labels.patch

# Download and cache dependencies
WORKDIR /vector/vector
RUN cargo fetch

RUN cargo build \
    --release \
    -j $(($(nproc) /2)) \
    --offline \
    --no-default-features \
    --features "api,api-client,enrichment-tables,sources-host_metrics,sources-internal_metrics,sources-file,sources-kubernetes_logs,transforms,sinks-prometheus,sinks-blackhole,sinks-elasticsearch,sinks-file,sinks-loki,sinks-socket,sinks-console,sinks-vector,unix,vendor-all,vrl-cli" \
    && strip target/release/vector

FROM $BASE_DEBIAN_BULLSEYE
RUN mkdir -p /etc/vector \
    && apt-get update \
    && apt-get install -yq ca-certificates tzdata inotify-tools gettext procps \
    && rm -rf /var/cache/apt/archives/*
COPY --from=build /vector/vector/target/release/vector /usr/bin/vector
COPY reloader /usr/bin/reloader
ENTRYPOINT ["/usr/bin/vector"]
