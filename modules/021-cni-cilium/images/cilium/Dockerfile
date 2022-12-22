# syntax=docker/dockerfile:1.2

# Copyright 2020-2021 Authors of Cilium
# SPDX-License-Identifier: Apache-2.0

# Images sources: https://github.com/cilium/cilium/blob/v1.12.4/images/runtime/Dockerfile

ARG BASE_GOLANG_18_BULLSEYE

# TODO: replace with some of base images when they become compatable
ARG UBUNTU_IMAGE=docker.io/library/ubuntu:22.04@sha256:27cb6e6ccef575a4698b66f5de06c7ecd61589132d5a91d098f7f3f9285415a9

ARG CILIUM_LLVM_IMAGE=quay.io/cilium/cilium-llvm:547db7ec9a750b8f888a506709adb41f135b952e@sha256:4d6fa0aede3556c5fb5a9c71bc6b9585475ac9b1064f516d4c45c8fb691c9d9e
ARG CILIUM_BPFTOOL_IMAGE=quay.io/cilium/cilium-bpftool:78448c1a37ff2b790d5e25c3d8b8ec3e96e6405f@sha256:99a9453a921a8de99899ef82e0822f0c03f65d97005c064e231c06247ad8597d
ARG CILIUM_IPROUTE2_IMAGE=quay.io/cilium/cilium-iproute2:3570d58349efb2d6b0342369a836998c93afd291@sha256:1abcd7a5d2117190ab2690a163ee9cd135bc9e4cf8a4df662a8f993044c79342

ARG CILIUM_BUILDER_IMAGE=quay.io/cilium/cilium-builder:f3ff491f1fb923136b8b5276fafd9d2ee460a265@sha256:764cc4a2ee14cdf57be3d4dbce132baa0fd7e62379ef6f6c05f3db4a7ccd64ba
ARG CILIUM_RUNTIME_IMAGE=cilium-runtime

FROM ${CILIUM_LLVM_IMAGE} as llvm-dist
FROM ${CILIUM_BPFTOOL_IMAGE} as bpftool-dist
FROM ${CILIUM_IPROUTE2_IMAGE} as iproute2-dist

FROM ${BASE_GOLANG_18_BULLSEYE} as gops-cni-builder

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

#
# cilium-envoy from github.com/cilium/proxy
#
# Image source: https://github.com/cilium/cilium/blob/v1.12.4/images/cilium/Dockerfile#L11
FROM quay.io/cilium/cilium-envoy:48cbb2f88edca93ba80d1135dbb067a827b8adc2@sha256:2c21f4a37bf6d46da88758a6194c0e63ab26c80a91c41d679247a921c46d42d3 as cilium-envoy

#
# Hubble CLI
#
FROM ${CILIUM_BUILDER_IMAGE} as hubble
RUN mkdir /tmp/cilium-repo && curl -sSL https://github.com/cilium/cilium/archive/refs/tags/v1.12.4.tar.gz | tar xvz -C /tmp/cilium-repo
RUN bash /tmp/cilium-repo/cilium-1.12.4/images/cilium/download-hubble.sh
RUN /out/linux/amd64/bin/hubble completion bash > /out/linux/bash_completion

FROM ${CILIUM_BUILDER_IMAGE} as builder

RUN apt-get update && apt-get install patch -y

RUN mkdir /tmp/cilium-repo && curl -sSL https://github.com/cilium/cilium/archive/refs/tags/v1.12.4.tar.gz | tar xvz -C /tmp/cilium-repo
WORKDIR /tmp/cilium-repo/cilium-1.12.4

COPY patches/001-netfilter-compatibility-mode.patch /

RUN patch -p1 < /001-netfilter-compatibility-mode.patch

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
