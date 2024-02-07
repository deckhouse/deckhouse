ARG BASE_UBUNTU
ARG BASE_RUST
ARG BASE_GOLANG_19_ALPINE

### 1: Build vector
FROM $BASE_RUST as build-vector

RUN apt-get update \
    && apt-get install -yq \
      ca-certificates make bash cmake libclang1-9 llvm-9 libsasl2-dev protobuf-compiler software-properties-common

# Install librdkafka-dev >=2.0 because bundled version (1.9.2) has bugs with CA certificates location.
# https://github.com/confluentinc/librdkafka/commit/f8830a28652532009e3f16854cb9d5004d9de06b
RUN apt-get update \
    && apt-get install -yq \
      wget build-essential libsasl2-dev libcurl4-openssl-dev libzstd-dev libssl-dev zlib1g-dev libc6-dev unzip

ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}

RUN git clone --depth 1 --branch v2.0.2 ${SOURCE_REPO}/confluentinc/librdkafka.git /librdkafka \
    && cd /librdkafka \
    && ./configure \
    && make \
    && make install

COPY patches/cef-encoder.patch /
WORKDIR /vector
RUN git clone --depth 1 --branch v0.31.0 ${SOURCE_REPO}/vectordotdev/vector.git \
    && cd vector \
    && git apply /cef-encoder.patch

# Download and cache dependencies
WORKDIR /vector/vector
RUN cargo fetch

RUN cargo build \
    --release \
    -j $(($(nproc) /2)) \
    --offline \
    --no-default-features \
    --features "api,api-client,enrichment-tables,sources-host_metrics,sources-internal_metrics,sources-file,sources-kubernetes_logs,transforms,sinks-prometheus,sinks-blackhole,sinks-elasticsearch,sinks-file,sinks-loki,sinks-socket,sinks-console,sinks-vector,sinks-kafka,sinks-splunk_hec,unix,rdkafka?/dynamic-linking,rdkafka?/gssapi-vendored" \
    && strip target/release/vector

### 2: Config reloader
FROM $BASE_GOLANG_19_ALPINE as artifact

ARG GOPROXY
ENV GOPROXY=${GOPROXY}

WORKDIR /src/
COPY reloader/ /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o reloader main.go

### 3: Final image
FROM $BASE_UBUNTU
RUN mkdir -p /etc/vector \
    && apt-get update \
    && apt-get install -yq ca-certificates tzdata inotify-tools gettext procps wget \
    && rm -rf /var/cache/apt/archives/*

# libssl.1
COPY lib/libssl1.1_1.1.1f-1ubuntu2.17_amd64.deb .
RUN dpkg -i libssl1.1_1.1.1f-1ubuntu2.17_amd64.deb && rm libssl1.1_1.1.1f-1ubuntu2.17_amd64.deb

# libcurl.4
RUN apt-get install -yq libcurl4

COPY --from=build-vector /vector/vector/target/release/vector /usr/bin/vector

COPY --from=build-vector /usr/local/lib/librdkafka.* /usr/local/lib/
COPY --from=build-vector /usr/local/lib/pkgconfig/rdkafka.pc /usr/local/lib/pkgconfig/rdkafka.pc

ENV LD_LIBRARY_PATH=/usr/local/lib

COPY --from=artifact /src/reloader /usr/bin/
ENTRYPOINT ["/usr/bin/vector"]
