ARG BASE_UBUNTU_BIONIC

FROM $BASE_UBUNTU_BIONIC as builder

WORKDIR /

RUN apt-get update && apt-get install -y --no-install-recommends build-essential fakeroot pbuilder aptitude git ca-certificates libelf1 software-properties-common

RUN git clone --branch=v0.22.0 --depth=1 https://github.com/iovisor/bcc.git /root/bcc && \
    git -C /root/bcc submodule update --init --recursive

RUN cd /root/bcc && \
    /usr/lib/pbuilder/pbuilder-satisfydepends && \
    PARALLEL=$(nproc) ./scripts/build-deb.sh release

RUN add-apt-repository ppa:longsleep/golang-backports && \
    apt-get install -y --no-install-recommends golang-1.17-go patch
ENV PATH="/usr/lib/go-1.17/bin:$PATH"

WORKDIR /ebpf-exporter

RUN wget -O - https://github.com/cloudflare/ebpf_exporter/archive/refs/tags/v1.2.5.tar.gz | tar -xzv --strip-components=1

COPY patches/001-kernel-version-detection-and-individual-program-failure.patch /
RUN patch -p1 < /001-kernel-version-detection-and-individual-program-failure.patch

RUN mv /root/bcc/libbcc_*.deb /tmp/libbcc.deb
RUN dpkg -i /tmp/libbcc.deb

RUN GOPROXY="off" GOFLAGS="-mod=vendor" go build -o /ebpf_exporter -v -ldflags=" \
    -X github.com/prometheus/common/version.Version=$(git describe) \
    -X github.com/prometheus/common/version.Branch=$(git rev-parse --abbrev-ref HEAD) \
    -X github.com/prometheus/common/version.Revision=$(git rev-parse --short HEAD) \
    -X github.com/prometheus/common/version.BuildUser=docker@$(hostname) \
    -X github.com/prometheus/common/version.BuildDate=$(date --iso-8601=seconds) \
    " ./cmd/ebpf_exporter


FROM $BASE_UBUNTU_BIONIC

RUN apt-get update && apt-get install -y --no-install-recommends libelf1 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /tmp/libbcc.deb /tmp/libbcc.deb
COPY --from=builder /ebpf_exporter /usr/local/bin/

RUN dpkg -i /tmp/libbcc.deb

ENTRYPOINT [ "/usr/local/bin/ebpf_exporter" ]
