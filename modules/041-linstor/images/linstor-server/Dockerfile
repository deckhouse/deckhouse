ARG BASE_UBUNTU
ARG BASE_GOLANG_19_BULLSEYE

FROM $BASE_UBUNTU as linstor-builder

ARG LINSTOR_GITREPO=https://github.com/LINBIT/linstor-server
ARG LINSTOR_VERSION=1.24.2

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update \
 && apt-get -y install build-essential debhelper git default-jdk-headless gradle python3-all wget zip \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# We are using Gradle version 6.9.3 because building Linstor Controller with version 7+ completes unsuccesfully.
RUN mkdir /opt/gradle && cd /opt/gradle \
 && wget -q https://services.gradle.org/distributions/gradle-6.9.3-bin.zip \
 && unzip gradle-6.9.3-bin.zip \
 && ln -s /opt/gradle/gradle-6.9.3/bin/gradle /usr/local/bin/gradle

# Copy patches
COPY ./patches /patches

RUN git clone ${LINSTOR_GITREPO} /linstor-server
WORKDIR /linstor-server
RUN git checkout v${LINSTOR_VERSION} \
 && git config --global user.email "builder@deckhouse.io" \
 && git config --global user.name "deckhouse" \
 && git am /patches/*.patch \
 && git tag -f v${LINSTOR_VERSION}

RUN git config --global url."https://github.com/".insteadOf git://github.com/ \
 && make debrelease \
 && rm -rf /root/.gradle/caches/ \
 && mv linstor-server-${LINSTOR_VERSION}.tar.gz /linstor-server_$(echo ${LINSTOR_VERSION} | tr '-' '~').orig.tar.gz \
 && tar -C / -xvf /linstor-server_$(echo ${LINSTOR_VERSION} | tr '-' '~').orig.tar.gz

WORKDIR /linstor-server-${LINSTOR_VERSION}
RUN dpkg-buildpackage -us -uc

# ------------------------------------------------------------------------------

FROM $BASE_UBUNTU as client-builder

ARG API_GITREPO=https://github.com/LINBIT/linstor-api-py
ARG API_VERSION=1.19.0
ARG CLIENT_GITREPO=https://github.com/LINBIT/linstor-client
ARG CLIENT_VERSION=1.19.0
ARG LINSTOR_COMMON_SUBMODULE_COMMIT=999c3c0793376f88b891eb7ef3dc8f8bc9aa26ec

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update \
 && apt-get -y install build-essential debhelper git python3-all python3-setuptools help2man bash-completion docbook-xsl xsltproc dh-python \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

RUN git clone --recurse-submodules ${API_GITREPO} /linstor-api-py
WORKDIR /linstor-api-py
RUN git checkout v${API_VERSION}
WORKDIR /linstor-api-py/linstor-common
RUN git checkout ${LINSTOR_COMMON_SUBMODULE_COMMIT}
WORKDIR /linstor-api-py
RUN make debrelease \
 && mv ./dist/python-linstor-${API_VERSION}.tar.gz ../python-linstor_$(echo ${API_VERSION} | tr '-' '~').orig.tar.gz \
 && tar -C / -xvf /python-linstor_$(echo ${API_VERSION} | tr '-' '~').orig.tar.gz
WORKDIR /python-linstor-${API_VERSION}
RUN dpkg-buildpackage -us -uc

RUN rm -rf /linstor-api-py \
 && mv /python-linstor-${API_VERSION} /linstor-api-py

RUN git clone ${CLIENT_GITREPO} /linstor-client
WORKDIR /linstor-client
RUN git checkout v${CLIENT_VERSION} \
 && make debrelease \
 && mv dist/linstor-client-${CLIENT_VERSION}.tar.gz /linstor-client_$(echo ${CLIENT_VERSION} | tr '-' '~').orig.tar.gz \
 && tar -C / -xvf /linstor-client_$(echo ${CLIENT_VERSION} | tr '-' '~').orig.tar.gz
WORKDIR /linstor-client-${CLIENT_VERSION}
RUN dpkg-buildpackage -us -uc

# ------------------------------------------------------------------------------

FROM $BASE_GOLANG_19_BULLSEYE as k8s-await-election-builder

ARG K8S_AWAIT_ELECTION_GITREPO=https://github.com/LINBIT/k8s-await-election
ARG K8S_AWAIT_ELECTION_VERSION=0.3.1

RUN git clone ${K8S_AWAIT_ELECTION_GITREPO} /usr/local/go/k8s-await-election/ \
 && cd /usr/local/go/k8s-await-election \
 && git reset --hard v${K8S_AWAIT_ELECTION_VERSION} \
 && make \
 && mv ./out/k8s-await-election-amd64 /k8s-await-election

# ------------------------------------------------------------------------------

FROM $BASE_UBUNTU as utils-builder

ARG UTILS_GITREPO=https://github.com/LINBIT/drbd-utils
ARG UTILS_VERSION=9.25.0

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

ARG THIN_SEND_RECV_GITREPO=https://github.com/LINBIT/thin-send-recv
ARG THIN_SEND_RECV_VERSION=1.0.1

RUN git clone --recurse-submodules ${THIN_SEND_RECV_GITREPO} /thin-send-recv
WORKDIR /thin-send-recv
RUN git checkout v${THIN_SEND_RECV_VERSION} \
 && make debrelease \
 && mv thin-send-recv-${THIN_SEND_RECV_VERSION}.tar.gz ../thin-send-recv_$(echo ${THIN_SEND_RECV_VERSION} | tr '-' '~').orig.tar.gz \
 && tar -C / -xvf ../thin-send-recv_$(echo ${THIN_SEND_RECV_VERSION} | tr '-' '~').orig.tar.gz
WORKDIR /thin-send-recv-${THIN_SEND_RECV_VERSION}
RUN dpkg-buildpackage -us -uc

# ------------------------------------------------------------------------------

FROM $BASE_UBUNTU
ARG PIRAEUS_GITREPO=https://github.com/piraeusdatastore/piraeus
ARG PIRAEUS_COMMIT_REF=20e96f83f52631dc06d011b7f96293d4026236a1

COPY --from=linstor-builder /linstor-common_*.deb /linstor-controller_*.deb /linstor-satellite_*.deb /packages/
COPY --from=client-builder /python-linstor_*.deb /linstor-client_*.deb /packages/
COPY --from=utils-builder /python-linstor_*.deb /drbd-utils_*.deb /thin-send-recv_*.deb /packages/
COPY --from=k8s-await-election-builder /k8s-await-election /usr/bin/k8s-await-election


# Source https://github.com/piraeusdatastore/piraeus/blob/master/dockerfiles/piraeus-server/Dockerfile
RUN { echo 'APT::Install-Recommends "false";' ; echo 'APT::Install-Suggests "false";' ; } > /etc/apt/apt.conf.d/99_piraeus && \
	# Enable contrib repos for zfsutils \
	 sed -r -i 's/^deb(.*)$/deb\1 contrib/' /etc/apt/sources.list && \
   apt-get update && \
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
    # lvm2: manage lvm storage pools \
      lvm2 \
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
	# zfsutils-linux: for zfs storage pools \
	  zfsutils-linux \
	 && \
	# remove udev, no need for it in the container \
	 apt-get remove -y udev \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* \
 && ln -sf /proc/mounts /etc/mtab \
 && sed -i 's/udev_rules.*=.*/udev_rules=0/ ; s/udev_sync.*=.*/udev_sync=0/ ; s/obtain_device_list_from_udev.*=.*/obtain_device_list_from_udev=0/' /etc/lvm/lvm.conf \
 && sed -i '/^devices {/a global_filter = [ "r|^/dev/drbd|" ]' /etc/lvm/lvm.conf


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


# controller
EXPOSE 3376/tcp 3377/tcp 3370/tcp 3371/tcp

# satellite
EXPOSE 3366/tcp 3367/tcp

RUN curl -Lfo /usr/bin/piraeus-entry.sh ${PIRAEUS_GITREPO}/raw/${PIRAEUS_COMMIT_REF}/dockerfiles/piraeus-server/entry.sh \
 && chmod +x /usr/bin/piraeus-entry.sh

# Add liveness probe script
COPY liveness.sh /liveness.sh
RUN chmod 755 /liveness.sh

CMD ["startSatellite"]
ENTRYPOINT ["/usr/bin/k8s-await-election", "/usr/bin/piraeus-entry.sh"]
