ARG BASE_UBUNTU
ARG BASE_RUST

### 1: Build librdkafka
FROM $BASE_UBUNTU as build-librdkafka

# Install librdkafka-dev >=2.0 because bundled version (1.9.2) has bugs with CA certificates location.
# https://github.com/confluentinc/librdkafka/commit/f8830a28652532009e3f16854cb9d5004d9de06b
RUN apt-get update \
    && apt-get install -yq \
      wget build-essential libsasl2-dev libcurl4-openssl-dev libzstd-dev libsasl2-dev libssl-dev zlib1g-dev libc6-dev unzip \
    && wget https://github.com/edenhill/librdkafka/archive/v2.0.2.zip \
    && unzip v2.0.2.zip \
    && cd librdkafka-2.0.2 \
    && ./configure \
    && make \
    && make install


### 2: Build vector
FROM $BASE_RUST as build-vector

RUN apt-get update \
    && apt-get install -yq \
      ca-certificates make bash cmake libclang1-9 llvm-9 libsasl2-dev protobuf-compiler software-properties-common

WORKDIR /vector
RUN git clone --depth 1 --branch v0.27.0 https://github.com/vectordotdev/vector.git \
    && cd vector

# Download and cache dependencies
WORKDIR /vector/vector
RUN cargo fetch

COPY --from=build-librdkafka /usr/local/lib/librdkafka.a /usr/local/include/librdkafka.a
COPY --from=build-librdkafka /usr/local/lib/librdkafka.so.1 /usr/local/include/librdkafka.so.1
COPY --from=build-librdkafka /usr/local/lib/pkgconfig/rdkafka.pc /usr/local/lib/pkgconfig/rdkafka.pc
RUN ln -s /usr/local/lib/librdkafka.so.1 /usr/local/lib/librdkafka.so

RUN cargo build \
    --release \
    -j $(($(nproc) /2)) \
    --offline \
    --no-default-features \
    --features "api,api-client,enrichment-tables,sources-host_metrics,sources-internal_metrics,sources-file,sources-kubernetes_logs,transforms,sinks-prometheus,sinks-blackhole,sinks-elasticsearch,sinks-file,sinks-loki,sinks-socket,sinks-console,sinks-vector,sinks-kafka,sinks-splunk_hec,unix,rdkafka?/dynamic-linking,rdkafka?/gssapi-vendored,vrl-cli" \
    && strip target/release/vector


### 3: Final image
FROM $BASE_UBUNTU
RUN mkdir -p /etc/vector \
    && apt-get update \
    && apt-get install -yq ca-certificates tzdata inotify-tools gettext procps \
    && rm -rf /var/cache/apt/archives/*

COPY --from=build-vector /vector/vector/target/release/vector /usr/bin/vector

COPY --from=build-librdkafka /usr/local/lib/librdkafka.a /usr/local/include/librdkafka.a
COPY --from=build-librdkafka /usr/local/lib/librdkafka.so.1 /usr/local/include/librdkafka.so.1
COPY --from=build-librdkafka /usr/local/lib/pkgconfig/rdkafka.pc /usr/local/lib/pkgconfig/rdkafka.pc
RUN ln -s /usr/local/lib/librdkafka.so.1 /usr/local/lib/librdkafka.so

COPY reloader /usr/bin/reloader
ENTRYPOINT ["/usr/bin/vector"]
