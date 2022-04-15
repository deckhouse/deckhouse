# syntax=docker/dockerfile:1.2

# Copyright 2020-2021 Authors of Cilium
# SPDX-License-Identifier: Apache-2.0

ARG CILIUM_BUILDER_IMAGE=quay.io/cilium/cilium-builder:fbbf2ed17db40c10f5da4344c53af4dea24931ae@sha256:635186226dfd1728c2391f1c325883b31f6de534cb58537a1343c67f9d059a7a
ARG CILIUM_RUNTIME_IMAGE=quay.io/cilium/cilium-runtime:da38890181c21062e752c7536c597fcae49a56de@sha256:947be28bb2fee51106a40fa8171c42256823be19c3109e492330d95e0cf19d08

# cilium-envoy from github.com/cilium/proxy
#
FROM quay.io/cilium/cilium-envoy:12e3081cc292764b1308668cab1e7e523429bedc@sha256:3b43de6e585376cc8037b84b2b4d37bdf41242253987e7ff7f72e4e200e61fb5 as cilium-envoy

#
# Hubble CLI
#
FROM ${CILIUM_BUILDER_IMAGE} as hubble
RUN mkdir /tmp/cilium-repo && curl -sSL https://github.com/cilium/cilium/archive/refs/tags/v1.12.0-rc1.tar.gz | tar xvz -C /tmp/cilium-repo
RUN bash /tmp/cilium-repo/cilium-1.12.0-rc1/images/cilium/download-hubble.sh
RUN /out/linux/amd64/bin/hubble completion bash > /out/linux/bash_completion

#
# Cilium incremental build. Should be fast given builder-deps is up-to-date!
#
# cilium-builder tag is the date on which the compatible build image
# was pushed.  If a new version of the build image is needed, it needs
# to be tagged with a new date and this file must be changed
# accordingly.  Keeping the old images available will allow older
# versions to be built while allowing the new versions to make changes
# that are not backwards compatible.
#
FROM ${CILIUM_BUILDER_IMAGE} as builder

RUN apt-get update && apt-get install patch -y

RUN mkdir /tmp/cilium-repo && curl -sSL https://github.com/cilium/cilium/archive/refs/tags/v1.12.0-rc1.tar.gz | tar xvz -C /tmp/cilium-repo
WORKDIR /tmp/cilium-repo/cilium-1.12.0-rc1

COPY 001-netfilter-compatibility-mode.patch /
COPY 002-skip-masquerad-only-for-internalip.patch /
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
