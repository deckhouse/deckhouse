ARG BASE_DEBIAN
ARG BASE_GOLANG_17_BUSTER

FROM $BASE_DEBIAN as linstor-builder

ARG LINSTOR_GITREPO=https://github.com/LINBIT/linstor-server
ARG LINSTOR_VERSION=1.18.0

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update \
 && apt-get -y install build-essential git default-jdk-headless dh-systemd gradle python3-all \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

RUN git clone ${LINSTOR_GITREPO} /linstor-server
WORKDIR /linstor-server
RUN git checkout v${LINSTOR_VERSION}

RUN git config --global url."https://github.com/".insteadOf git://github.com/ \
 && make debrelease \
 && rm -rf /root/.gradle/caches/ \
 && mv linstor-server-${LINSTOR_VERSION}.tar.gz /linstor-server_$(echo ${LINSTOR_VERSION} | tr '-' '~').orig.tar.gz \
 && tar -C / -xvf /linstor-server_$(echo ${LINSTOR_VERSION} | tr '-' '~').orig.tar.gz

WORKDIR /linstor-server-${LINSTOR_VERSION}
RUN dpkg-buildpackage -us -uc

# ------------------------------------------------------------------------------

FROM $BASE_DEBIAN as client-builder

ARG API_GITREPO=https://github.com/LINBIT/linstor-api-py
ARG API_VERSION=1.13.0
ARG CLIENT_GITREPO=https://github.com/LINBIT/linstor-client
ARG CLIENT_VERSION=1.13.0

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update \
 && apt-get -y install build-essential debhelper git python3-all python3-setuptools help2man bash-completion docbook-xsl xsltproc \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

RUN git clone --recurse-submodules ${API_GITREPO} /linstor-api-py
WORKDIR /linstor-api-py
RUN git checkout v${API_VERSION} \
 && make debrelease \
 && mv ./dist/python-linstor-${API_VERSION}.tar.gz ../python-linstor_${API_VERSION}.orig.tar.gz \
 && tar -C / -xvf /python-linstor_${API_VERSION}.orig.tar.gz
WORKDIR /python-linstor-${API_VERSION}
RUN dpkg-buildpackage -us -uc

RUN rm -rf /linstor-api-py \
 && mv /python-linstor-${API_VERSION} /linstor-api-py

RUN git clone ${CLIENT_GITREPO} /linstor-client
WORKDIR /linstor-client
RUN git checkout v${CLIENT_VERSION} \
 && make debrelease \
 && mv dist/linstor-client-${CLIENT_VERSION}.tar.gz /linstor-client_${CLIENT_VERSION}.orig.tar.gz \
 && tar -C / -xvf /linstor-client_${CLIENT_VERSION}.orig.tar.gz
WORKDIR /linstor-client-${CLIENT_VERSION}
RUN dpkg-buildpackage -us -uc

# ------------------------------------------------------------------------------

FROM $BASE_GOLANG_17_BUSTER as k8s-await-election-builder

ARG K8S_AWAIT_ELECTION_GITREPO=https://github.com/LINBIT/k8s-await-election
ARG K8S_AWAIT_ELECTION_VERSION=0.3.1

RUN git clone ${K8S_AWAIT_ELECTION_GITREPO} /usr/local/go/k8s-await-election/ \
 && cd /usr/local/go/k8s-await-election \
 && git reset --hard v${K8S_AWAIT_ELECTION_VERSION} \
 && make \
 && mv ./out/k8s-await-election-amd64 /k8s-await-election

# ------------------------------------------------------------------------------

FROM $BASE_DEBIAN as utils-builder

ARG UTILS_GITREPO=https://github.com/LINBIT/drbd-utils
ARG UTILS_VERSION=9.20.2

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update \
 && apt-get -y install build-essential debhelper git flex clitest xsltproc locales libxml2-utils po4a bash-completion dh-systemd docbook-xsl udev asciidoctor \
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
 && mv drbd-utils-${UTILS_VERSION}.tar.gz ../drbd-utils_${UTILS_VERSION}.orig.tar.gz \
 && tar -C / -xvf ../drbd-utils_${UTILS_VERSION}.orig.tar.gz
WORKDIR /drbd-utils-${UTILS_VERSION}
RUN dpkg-buildpackage -us -uc

ARG THIN_SEND_RECV_GITREPO=https://github.com/LINBIT/thin-send-recv
ARG THIN_SEND_RECV_VERSION=1.0.1

RUN git clone --recurse-submodules ${THIN_SEND_RECV_GITREPO} /thin-send-recv
WORKDIR /thin-send-recv
RUN git checkout v${THIN_SEND_RECV_VERSION} \
 && make debrelease \
 && mv thin-send-recv-${THIN_SEND_RECV_VERSION}.tar.gz ../thin-send-recv_${THIN_SEND_RECV_VERSION}.orig.tar.gz \
 && tar -C / -xvf ../thin-send-recv_${THIN_SEND_RECV_VERSION}.orig.tar.gz
WORKDIR /thin-send-recv-${THIN_SEND_RECV_VERSION}
RUN dpkg-buildpackage -us -uc

# ------------------------------------------------------------------------------

FROM $BASE_DEBIAN
ARG PIRAEUS_COMMIT_REF=c0ccd4ecb07933ee5af2627bd3b8d37d0702f23c

COPY --from=linstor-builder /linstor-common_*.deb /linstor-controller_*.deb /linstor-satellite_*.deb /packages/
COPY --from=client-builder /python-linstor_*.deb /linstor-client_*.deb /packages/
COPY --from=utils-builder /python-linstor_*.deb /drbd-utils_*.deb /thin-send-recv_*.deb /packages/
COPY --from=k8s-await-election-builder /k8s-await-election /usr/bin/k8s-await-election

# Source https://github.com/piraeusdatastore/piraeus/blob/master/dockerfiles/piraeus-server/Dockerfile
RUN { echo 'APT::Install-Recommends "false";' ; echo 'APT::Install-Suggests "false";' ; } > /etc/apt/apt.conf.d/99_piraeus
RUN apt-get update && \
	# Install useful utilities and general dependencies
	 apt-get install -y udev drbd-utils jq net-tools iputils-ping iproute2 dnsutils netcat sysstat curl util-linux && \
	# Install dependencies for optional features \
	 apt-get install -y \
	# cryptsetup: luks layer
	  cryptsetup \
	# e2fsprogs: LINSTOR can create file systems \
	  e2fsprogs \
	# lsscsi: exos layer \
	  lsscsi \
	# multipath-tools: exos layer \
	  multipath-tools \
	# nvme-cli: nvme layer
	  nvme-cli \
	# procps: used by LINSTOR to find orphaned send/receive processes \
	  procps \
	# socat: used with thin-send-recv to send snapshots to another LINSTOR cluster
	  socat \
	# xfsprogs: LINSTOR can create file systems; xfs deps \
	  xfsprogs \
	# zstd: used with thin-send-recv to send snapshots to another LINSTOR cluster \
	  zstd \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# Install linstor-server
RUN apt-get update \
 && apt-get install -y default-jre-headless thin-provisioning-tools python3-all python3-natsort \
 && dpkg -i packages/*.deb \
 && sed -i 's/usage-count yes;/usage-count no;/' /etc/drbd.d/global_common.conf \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# Log directory need to be group writable. OpenShift assigns random UID and GID, without extra RBAC changes we can only influence the GID.
RUN mkdir /var/log/linstor-controller && \
	 chown 0:1000 /var/log/linstor-controller && \
	 chmod -R 0775 /var/log/linstor-controller && \
	 # Ensure we log to files in containers, otherwise SOS reports won't show any logs at all
	 sed -i 's#<!-- <appender-ref ref="FILE" /> -->#<appender-ref ref="FILE" />#' /usr/share/linstor-server/lib/conf/logback.xml

# needs a newer lvm, debian bug #932433
RUN printf 'Package: *\nPin: release a=%s\nPin-Priority: %s\n\n' stable 700 testing 650 > /etc/apt/preferences \
 && echo 'deb http://deb.debian.org/debian/ bullseye main contrib' >> /etc/apt/sources.list \
 && apt-get update \
 && apt-get install -y -t bullseye lvm2 \
 && apt-get remove -y udev \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* \
 && ln -sf /proc/mounts /etc/mtab \
 && sed -i 's/udev_rules.*=.*/udev_rules=0/ ; s/udev_sync.*=.*/udev_sync=0/ ; s/obtain_device_list_from_udev.*=.*/obtain_device_list_from_udev=0/' /etc/lvm/lvm.conf \
 && sed -i '/^devices {/a global_filter = [ "r|^/dev/drbd|" ]' /etc/lvm/lvm.conf

# controller
EXPOSE 3376/tcp 3377/tcp 3370/tcp 3371/tcp

# satellite
EXPOSE 3366/tcp 3367/tcp

RUN curl -Lfo /usr/bin/piraeus-entry.sh https://github.com/piraeusdatastore/piraeus/raw/${PIRAEUS_COMMIT_REF}/dockerfiles/piraeus-server/entry.sh \
 && chmod +x /usr/bin/piraeus-entry.sh

CMD ["startSatellite"]
ENTRYPOINT ["/usr/bin/k8s-await-election", "/usr/bin/piraeus-entry.sh"]
