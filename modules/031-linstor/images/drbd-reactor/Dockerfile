ARG BASE_DEBIAN

FROM $BASE_DEBIAN as utils-builder

ARG UTILS_VERSION=9.20.2

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update \
 && apt-get -y install build-essential debhelper git flex clitest xsltproc locales libxml2-utils po4a bash-completion dh-systemd docbook-xsl udev asciidoctor \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

RUN git clone --recurse-submodules https://github.com/LINBIT/drbd-utils /drbd-utils
WORKDIR /drbd-utils
RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    locale-gen
ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8
RUN git checkout v${UTILS_VERSION} \
 && git submodule update --init --force --checkout \
 && install /dev/null /usr/local/bin/lbvers.py \
 && ./autogen.sh \
 && ./configure \
 && make debrelease VERSION=${UTILS_VERSION} \
 && mv drbd-utils-${UTILS_VERSION}.tar.gz ../drbd-utils_${UTILS_VERSION}.orig.tar.gz \
 && tar -C / -xvf ../drbd-utils_${UTILS_VERSION}.orig.tar.gz
WORKDIR /drbd-utils-${UTILS_VERSION}
RUN dpkg-buildpackage -us -uc

# ------------------------------------------------------------------------------

FROM $BASE_DEBIAN as reactor-builder

ARG REACTOR_VERSION=0.5.3

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update \
 && apt-get -y install build-essential debhelper git cargo rustc dh-python \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

RUN git clone --recurse-submodules https://github.com/LINBIT/drbd-reactor /drbd-reactor
WORKDIR /drbd-reactor
RUN git checkout v${REACTOR_VERSION} \
 && install /dev/null /usr/local/bin/lbvers.py \
 && make debrelease VERSION=${REACTOR_VERSION} \
 && mv drbd-reactor-${REACTOR_VERSION}.tar.gz ../drbd-reactor_${REACTOR_VERSION}.orig.tar.gz \
 && tar -C / -xvf ../drbd-reactor_${REACTOR_VERSION}.orig.tar.gz
WORKDIR /drbd-reactor-${REACTOR_VERSION}
RUN dpkg-buildpackage -us -uc

# ------------------------------------------------------------------------------

FROM $BASE_DEBIAN

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update \
 && apt-get -y install python3-toml \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

COPY --from=utils-builder /python-linstor_*.deb /drbd-utils_*.deb /thin-send-recv_*.deb /packages/
COPY --from=reactor-builder /drbd-reactor_*.deb /packages/

RUN dpkg -i packages/*.deb

ENTRYPOINT ["/usr/sbin/drbd-reactor"]
