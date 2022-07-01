ARG BASE_DEBIAN_BULLSEYE

FROM $BASE_DEBIAN_BULLSEYE as builder
ARG DRBD_GITREPO=https://github.com/LINBIT/drbd
ARG DRBD_VERSION=9.1.7

RUN apt-get update \
 && apt-get install -y make git \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

RUN git clone ${DRBD_GITREPO} /drbd \
 && cd /drbd \
 && git reset --hard drbd-${DRBD_VERSION} \
 && make tarball \
 && mv ./drbd-*.tar.gz /drbd.tar.gz \
 && mv ./docker/entry.sh /entry.sh \
 && chmod +x /entry.sh

FROM $BASE_DEBIAN_BULLSEYE

RUN apt-get update \
 && apt-get install -y kmod gnupg wget make gcc patch curl coccinelle \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /entry.sh /drbd.tar.gz /
ENV SPAAS=false

ENV LB_HOW compile
ENTRYPOINT [ "/entry.sh" ]
