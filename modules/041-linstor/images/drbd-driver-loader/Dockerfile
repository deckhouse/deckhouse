ARG BASE_UBUNTU

FROM $BASE_UBUNTU as builder
ARG DRBD_GITREPO=https://github.com/LINBIT/drbd
ARG DRBD_VERSION=9.2.1

RUN apt-get update \
 && apt-get install -y make git \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# Using source code from GitHub repository
RUN git clone ${DRBD_GITREPO} /drbd \
 && cd /drbd \
 && git reset --hard drbd-${DRBD_VERSION} \
 && make tarball \
 && mv ./drbd-*.tar.gz /drbd.tar.gz

# # Using source code provided by LINBIT
# RUN DRBD_PKG=https://pkg.linbit.com//downloads/drbd/9/drbd-${DRBD_VERSION}.tar.gz \
#  && wget -O /drbd.tar.gz ${DRBD_PKG}

RUN tar xvf /drbd.tar.gz --strip-components=2 --wildcards 'drbd-*/docker/entry.sh' \
 && chmod +x /entry.sh

FROM $BASE_UBUNTU

RUN apt-get update \
 && apt-get install -y kmod gnupg wget make gcc patch curl libelf-dev \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /entry.sh /drbd.tar.gz /
ENV SSL_CERT_FILE=/etc/spaas/certs/ca.crt

ENV LB_HOW compile
ENTRYPOINT [ "/entry.sh" ]
