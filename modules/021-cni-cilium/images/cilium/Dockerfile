# syntax=docker/dockerfile:1.2

# Copyright 2020-2021 Authors of Cilium
# SPDX-License-Identifier: Apache-2.0

ARG GOLANG_IMAGE=docker.io/library/golang:1.17.9@sha256:c1bf2101f7e1e0b08ce8f792735a81cd71b9d81cbcdceef2382140694f1ffbaf
ARG UBUNTU_IMAGE=docker.io/library/ubuntu:20.04@sha256:cf31af331f38d1d7158470e095b132acd126a7180a54f263d386da88eb681d93

ARG CILIUM_LLVM_IMAGE=quay.io/cilium/cilium-llvm:0147a23fdada32bd51b4f313c645bcb5fbe188d6@sha256:24fd3ad32471d0e45844c856c38f1b2d4ac8bd0a2d4edf64cffaaa3fd0b21202
ARG CILIUM_BPFTOOL_IMAGE=quay.io/cilium/cilium-bpftool:7a5420acb4a0fa276a549eb4674515eadb2821d7@sha256:3e204885a1b9ec2a5b593584608664721ef0bd221d3920c091c2e77eb259090c
ARG CILIUM_IPROUTE2_IMAGE=quay.io/cilium/cilium-iproute2:824df0c341c724f4b12cc48762f80aa3d698b589@sha256:0af5e059b2a43c6383a3daa293182b50cb88f7761f543dacf43c1c3f8f79030c

ARG CILIUM_BUILDER_IMAGE=quay.io/cilium/cilium-builder:a5ba9ba04c230e4fe86c1eac885de7a911e9ce74@sha256:cb302f02973c69231c3a8837c950ac3ed1c1a1216404d46b9b786e033fed4351
ARG CILIUM_RUNTIME_IMAGE=cilium-runtime

FROM ${CILIUM_LLVM_IMAGE} as llvm-dist
FROM ${CILIUM_BPFTOOL_IMAGE} as bpftool-dist
FROM ${CILIUM_IPROUTE2_IMAGE} as iproute2-dist

FROM ${GOLANG_IMAGE} as gops-cni-builder

RUN apt-get update && apt-get install -y binutils-aarch64-linux-gnu binutils-x86-64-linux-gnu

WORKDIR /go/src/github.com/cilium/cilium/images/runtime

COPY build-gops.sh .
RUN ./build-gops.sh

COPY download-cni.sh .
COPY cni-version.sh .
RUN ./download-cni.sh

FROM ${UBUNTU_IMAGE} as rootfs

# Update ubuntu packages to the most recent versions
RUN apt-get update && \
    apt-get upgrade -y

WORKDIR /go/src/github.com/cilium/cilium/images/runtime

COPY install-runtime-deps.sh .
RUN ./install-runtime-deps.sh

COPY configure-iptables-wrapper.sh .
COPY iptables-wrapper /usr/sbin/iptables-wrapper
RUN ./configure-iptables-wrapper.sh

COPY --from=llvm-dist /usr/local/bin/clang /usr/local/bin/llc /bin/
COPY --from=bpftool-dist /usr/local /usr/local
COPY --from=iproute2-dist /usr/lib/libbpf* /usr/lib/
COPY --from=iproute2-dist /usr/local /usr/local

COPY --from=gops-cni-builder /out/linux/amd64/bin/loopback /cni/loopback
COPY --from=gops-cni-builder /out/linux/amd64/bin/gops /bin/gops


FROM scratch as cilium-runtime
LABEL maintainer="maintainer@cilium.io"
COPY --from=rootfs / /


# cilium-envoy from github.com/cilium/proxy
#
FROM quay.io/cilium/cilium-envoy:9c0d933166ba192713f9e2fc3901f788557286ee@sha256:943f1f522bdfcb1ca3fe951bd8186c41b970afa254096513ae6e0e0efda1a10d as cilium-envoy

#
# Hubble CLI
#
FROM ${CILIUM_BUILDER_IMAGE} as hubble
RUN mkdir /tmp/cilium-repo && curl -sSL https://github.com/cilium/cilium/archive/refs/tags/v1.11.5.tar.gz | tar xvz -C /tmp/cilium-repo
RUN bash /tmp/cilium-repo/cilium-1.11.5/images/cilium/download-hubble.sh
RUN /out/linux/amd64/bin/hubble completion bash > /out/linux/bash_completion

FROM ${CILIUM_BUILDER_IMAGE} as builder

RUN apt-get update && apt-get install patch -y

RUN mkdir /tmp/cilium-repo && curl -sSL https://github.com/cilium/cilium/archive/refs/tags/v1.11.5.tar.gz | tar xvz -C /tmp/cilium-repo
WORKDIR /tmp/cilium-repo/cilium-1.11.5

COPY patches/001-netfilter-compatibility-mode.patch /
COPY patches/002-skip-host-ip-gc.patch /
RUN patch -p1 < /001-netfilter-compatibility-mode.patch && \
    patch -p1 < /002-skip-host-ip-gc.patch

RUN make PKG_BUILD=1 \
    SKIP_DOCS=true DESTDIR=/tmp/install build-container install-container-binary

RUN make PKG_BUILD=1 \
    SKIP_DOCS=true DESTDIR=/tmp/install install-bash-completion licenses-all && \
    mv LICENSE.all /tmp/install/LICENSE.all

RUN cp -t /tmp/install images/cilium/init-container.sh \
     plugins/cilium-cni/cni-install.sh \
     plugins/cilium-cni/cni-uninstall.sh

#
# Cilium runtime install.
#
# cilium-runtime tag is a date on which the compatible runtime base
# was pushed.  If a new version of the runtime is needed, it needs to
# be tagged with a new date and this file must be changed accordingly.
# Keeping the old runtimes available will allow older versions to be
# built while allowing the new versions to make changes that are not
# backwards compatible.
#
FROM ${CILIUM_RUNTIME_IMAGE}
RUN groupadd -f cilium \
    && echo ". /etc/profile.d/bash_completion.sh" >> /etc/bash.bashrc
COPY --from=cilium-envoy / /
# When used within the Cilium container, Hubble CLI should target the
# local unix domain socket instead of Hubble Relay.
ENV HUBBLE_SERVER=unix:///var/run/cilium/hubble.sock
COPY --from=hubble /out/linux/amd64/bin/hubble /usr/bin/hubble
COPY --from=hubble /out/linux/bash_completion /etc/bash_completion.d/hubble

COPY --from=builder /tmp/install /
WORKDIR /home/cilium

ENV INITSYSTEM="SYSTEMD"
CMD ["/usr/bin/cilium"]
