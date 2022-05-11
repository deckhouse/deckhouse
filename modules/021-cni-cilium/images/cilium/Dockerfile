# syntax=docker/dockerfile:1.2

# Copyright 2020-2021 Authors of Cilium
# SPDX-License-Identifier: Apache-2.0

ARG GOLANG_IMAGE=docker.io/library/golang:1.18.0@sha256:b5a9ae5907066849cbb2b47af93713b1f030e3eb6b36d84f2ae5f354b25c80d4
ARG UBUNTU_IMAGE=docker.io/library/ubuntu:20.04@sha256:626ffe58f6e7566e00254b638eb7e0f3b11d4da9675088f4781a50ae288f3322

ARG CILIUM_LLVM_IMAGE=quay.io/cilium/cilium-llvm:547db7ec9a750b8f888a506709adb41f135b952e@sha256:4d6fa0aede3556c5fb5a9c71bc6b9585475ac9b1064f516d4c45c8fb691c9d9e
ARG CILIUM_BPFTOOL_IMAGE=quay.io/cilium/cilium-bpftool:523f6b3928c309e0ee5ebbc2241604892912e507@sha256:4c31054bf0bf252117ef0e1b3d09c3a607baa7f06425b0d6432124a5333319c9
ARG CILIUM_IPROUTE2_IMAGE=quay.io/cilium/cilium-iproute2:02c29c971c01f0b9a7b916327f0caedd83820c18@sha256:eeb019043163891b91b731bed237d1edfaac036993b75f839b7d8fe6dd82b866

ARG CILIUM_BUILDER_IMAGE=quay.io/cilium/cilium-builder:9b4e9adc12f53046ee52059e9f5ed1b5e7c902e9@sha256:7bed24afabb3b88eb39f6d6a391be08dc085e32bee43573c63ade623febc3292
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
RUN mkdir /tmp/cilium-repo && curl -sSL https://github.com/cilium/cilium/archive/refs/tags/v1.11.4.tar.gz | tar xvz -C /tmp/cilium-repo
RUN bash /tmp/cilium-repo/cilium-1.11.4/images/cilium/download-hubble.sh
RUN /out/linux/amd64/bin/hubble completion bash > /out/linux/bash_completion

FROM ${CILIUM_BUILDER_IMAGE} as builder

RUN apt-get update && apt-get install patch -y

RUN mkdir /tmp/cilium-repo && curl -sSL https://github.com/cilium/cilium/archive/refs/tags/v1.11.4.tar.gz | tar xvz -C /tmp/cilium-repo
WORKDIR /tmp/cilium-repo/cilium-1.11.4

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
