ARG BASE_ALPINE_EDGE

FROM $BASE_ALPINE_EDGE as builder

WORKDIR /

RUN apk add go git build-base linux-headers \
    libbpf-dev libelf-static zlib-static

RUN git clone --depth=1 --branch=v2.0.0 https://github.com/cloudflare/ebpf_exporter.git
WORKDIR /ebpf_exporter

#TODO Adapt 001-kernel-version-detection-and-individual-program-failure.patch & fix incorrect 'date' call
RUN make build

FROM $BASE_ALPINE_EDGE

COPY --from=builder /ebpf_exporter/ebpf_exporter /usr/local/bin/

ENTRYPOINT [ "/usr/local/bin/ebpf_exporter" ]
