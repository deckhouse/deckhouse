ARG BASE_UBUNTU
ARG BASE_GOLANG_18_BULLSEYE

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

FROM $BASE_GOLANG_18_BULLSEYE as builder
ARG PIRAEUS_HA_CONTROLLER_GITREPO=https://github.com/piraeusdatastore/piraeus-ha-controller
ARG PIRAEUS_HA_CONTROLLER_VERSION=1.1.1

RUN git clone ${PIRAEUS_HA_CONTROLLER_GITREPO} /usr/local/go/piraeus-ha-controller \
 && cd /usr/local/go/piraeus-ha-controller \
 && git reset --hard v${PIRAEUS_HA_CONTROLLER_VERSION} \
 && cd cmd/agent \
 && go build -ldflags="-X github.com/piraeusdatastore/piraeus-ha-controller/pkg/metadata.Version=v${PIRAEUS_HA_CONTROLLER_VERSION}" \
 && mv ./agent /

# ------------------------------------------------------------------------------

FROM $BASE_UBUNTU
COPY --from=utils-builder /python-linstor_*.deb /drbd-utils_*.deb /packages/
COPY --from=builder /agent /

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update \
 && apt-get -y install python3-toml \
 && dpkg -i packages/*.deb \
 && sed -i 's/usage-count yes;/usage-count no;/' /etc/drbd.d/global_common.conf \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["/agent"]
