# syntax=docker/dockerfile:1.2

# Copyright 2020-2021 Authors of Cilium
# SPDX-License-Identifier: Apache-2.0

ARG CILIUM_BUILDER_IMAGE=quay.io/cilium/cilium-builder:c2ec83b1a4884d37ca89510cb9dceb8d741a0cf9@sha256:105cba5bd854763f83a74ac963b8c828770a92819c2da032a10e2ea6d909ab5b
ARG CILIUM_RUNTIME_IMAGE=quay.io/cilium/cilium-runtime:d254efbeee6254829e016e8ce1fd1a4a98a11847@sha256:d69cec23a40bfb2f6541256d0b59838e22f31343b977ef5dbd0189114cd9bcd4

# cilium-envoy from github.com/cilium/proxy
#
FROM quay.io/cilium/cilium-envoy:9c0d933166ba192713f9e2fc3901f788557286ee@sha256:943f1f522bdfcb1ca3fe951bd8186c41b970afa254096513ae6e0e0efda1a10d as cilium-envoy

#
# Hubble CLI
#
FROM ${CILIUM_BUILDER_IMAGE} as hubble
RUN mkdir /tmp/cilium-repo && curl -sSL https://github.com/cilium/cilium/archive/refs/tags/v1.11.2.tar.gz | tar xvz -C /tmp/cilium-repo
RUN bash /tmp/cilium-repo/cilium-1.11.2/images/cilium/download-hubble.sh
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

RUN mkdir /tmp/cilium-repo && curl -sSL https://github.com/cilium/cilium/archive/refs/tags/v1.11.2.tar.gz | tar xvz -C /tmp/cilium-repo
WORKDIR /tmp/cilium-repo/cilium-1.11.2

COPY 001-skip-masquerade-only-for-internalip.patch /
RUN patch -p1 < /001-skip-masquerade-only-for-internalip.patch

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
