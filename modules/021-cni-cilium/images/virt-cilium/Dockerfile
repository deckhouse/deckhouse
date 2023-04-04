# syntax=docker/dockerfile:1.2

# https://github.com/cilium/cilium/blob/v1.12.8/images/runtime/Dockerfile#L7-L12
ARG TESTER_IMAGE=quay.io/cilium/image-tester:c37f768323abfba87c90cd9c82d37136183457bc@sha256:4c9d640b6379eb4964b8590acc95ca2dfaa71df70f4467fb7d8ac076acf6a8e1
ARG GOLANG_IMAGE=docker.io/library/golang:1.18.10@sha256:50c889275d26f816b5314fc99f55425fa76b18fcaf16af255f5d57f09e1f48da
ARG UBUNTU_IMAGE=docker.io/library/ubuntu:20.04@sha256:9fa30fcef427e5e88c76bc41ad37b7cc573e1d79cecb23035e413c4be6e476ab

ARG CILIUM_LLVM_IMAGE=quay.io/cilium/cilium-llvm:547db7ec9a750b8f888a506709adb41f135b952e@sha256:4d6fa0aede3556c5fb5a9c71bc6b9585475ac9b1064f516d4c45c8fb691c9d9e
ARG CILIUM_BPFTOOL_IMAGE=quay.io/cilium/cilium-bpftool:78448c1a37ff2b790d5e25c3d8b8ec3e96e6405f@sha256:99a9453a921a8de99899ef82e0822f0c03f65d97005c064e231c06247ad8597d
ARG CILIUM_IPROUTE2_IMAGE=quay.io/cilium/cilium-iproute2:3570d58349efb2d6b0342369a836998c93afd291@sha256:1abcd7a5d2117190ab2690a163ee9cd135bc9e4cf8a4df662a8f993044c79342

# https://github.com/cilium/cilium/blob/v1.12.8/images/cilium/Dockerfile#L6
ARG CILIUM_BUILDER_IMAGE=quay.io/cilium/cilium-builder:4b33680817b91739f671f747df01b7878aece385@sha256:870e3f356e716a7b5957a2a5974d206442f85b78cb4f65aa3320c5bed27d27d5
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
    apt-get upgrade -y && \
    apt-get install -y jq

WORKDIR /go/src/github.com/cilium/cilium/images/runtime

COPY install-runtime-deps.sh .
RUN ./install-runtime-deps.sh

COPY iptables-wrapper-installer.sh .
RUN ./iptables-wrapper-installer.sh

COPY --from=llvm-dist /usr/local/bin/clang /usr/local/bin/llc /bin/
COPY --from=bpftool-dist /usr/local /usr/local
COPY --from=iproute2-dist /usr/lib/libbpf* /usr/lib/
COPY --from=iproute2-dist /usr/local /usr/local

COPY --from=gops-cni-builder /out/linux/amd64/bin/loopback /cni/loopback
COPY --from=gops-cni-builder /out/linux/amd64/bin/gops /bin/gops


FROM scratch as cilium-runtime
COPY --from=rootfs / /


# cilium-envoy from github.com/cilium/proxy
#
# https://github.com/cilium/cilium/blob/v1.12.8/images/cilium/Dockerfile#L11
FROM quay.io/cilium/cilium-envoy:04413917ff99e4f6ab51d1c6eb424d4a055f4462@sha256:af076f80818bc8d894f2f4f3104d5f4288112a67be5fb6e1b9a9c78370c7c9c8 as cilium-envoy

#
# Cilium builder image with tools and source code
#
FROM ${CILIUM_BUILDER_IMAGE} as cilium_builder
RUN apt-get update && apt-get install patch curl -y
RUN mkdir /tmp/cilium-repo && curl -sSL https://github.com/cilium/cilium/archive/refs/tags/v1.12.8.tar.gz | tar xvz -C /tmp/cilium-repo

#
# Hubble CLI
#
FROM cilium_builder as hubble
RUN bash /tmp/cilium-repo/cilium-1.12.8/images/cilium/download-hubble.sh
RUN /out/linux/amd64/bin/hubble completion bash > /out/linux/bash_completion

FROM cilium_builder as builder

WORKDIR /tmp/cilium-repo/cilium-1.12.8

# Apply patches
COPY ./patches /patches
RUN find /patches -name '*.patch' -exec git apply {} \;

RUN make PKG_BUILD=1 \
    SKIP_DOCS=true DESTDIR=/tmp/install build-container install-container-binary

RUN make PKG_BUILD=1 \
    SKIP_DOCS=true DESTDIR=/tmp/install install-bash-completion licenses-all && \
    mv LICENSE.all /tmp/install/LICENSE.all

RUN cp -t /tmp/install images/cilium/init-container.sh \
     plugins/cilium-cni/cni-install.sh \
     plugins/cilium-cni/install-plugin.sh \
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
