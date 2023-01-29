ARG BASE_UBUNTU

FROM $BASE_UBUNTU as utils-builder

ARG UTILS_GITREPO=https://github.com/LINBIT/drbd-utils
ARG UTILS_VERSION=9.23.0

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update \
 && apt-get -y install build-essential debhelper git flex clitest xsltproc locales libxml2-utils po4a bash-completion docbook-xsl udev asciidoctor \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

RUN git clone --recurse-submodules ${UTILS_GITREPO} /drbd-utils
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
 && mv drbd-utils-${UTILS_VERSION}.tar.gz ../drbd-utils_$(echo ${UTILS_VERSION} | tr '-' '~').orig.tar.gz \
 && tar -C / -xvf ../drbd-utils_$(echo ${UTILS_VERSION} | tr '-' '~').orig.tar.gz
WORKDIR /drbd-utils-${UTILS_VERSION}
RUN dpkg-buildpackage -us -uc

# ------------------------------------------------------------------------------

FROM $BASE_UBUNTU as reactor-builder

ARG REACTOR_GITREPO=https://github.com/LINBIT/drbd-reactor
ARG REACTOR_VERSION=1.0.0

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update \
 && apt-get -y install build-essential debhelper git cargo rustc dh-python bash-completion \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

RUN git clone --recurse-submodules ${REACTOR_GITREPO} /drbd-reactor
WORKDIR /drbd-reactor
RUN git reset --hard v${REACTOR_VERSION} \
 && install /dev/null /usr/local/bin/lbvers.py \
 && make debrelease VERSION=${REACTOR_VERSION} \
 && mv drbd-reactor-${REACTOR_VERSION}.tar.gz ../drbd-reactor_$(echo ${REACTOR_VERSION} | tr '-' '~').orig.tar.gz \
 && tar -C / -xvf ../drbd-reactor_$(echo ${REACTOR_VERSION} | tr '-' '~').orig.tar.gz
WORKDIR /drbd-reactor-${REACTOR_VERSION}
RUN dpkg-buildpackage -us -uc

# ------------------------------------------------------------------------------

FROM $BASE_UBUNTU

COPY --from=utils-builder /python-linstor_*.deb /drbd-utils_*.deb /thin-send-recv_*.deb /packages/
COPY --from=reactor-builder /drbd-reactor_*.deb /packages/

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update \
 && apt-get -y install python3-toml \
 && dpkg -i packages/*.deb \
 && sed -i 's/usage-count yes;/usage-count no;/' /etc/drbd.d/global_common.conf \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["/usr/sbin/drbd-reactor"]
