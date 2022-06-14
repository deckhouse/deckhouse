ARG BASE_DEBIAN_BULLSEYE

FROM $BASE_DEBIAN_BULLSEYE as builder
ARG DRBD_GITREPO=https://github.com/LINBIT/drbd
ARG DRBD_VERSION=9.1.7

RUN apt-get update \
 && apt-get install -y make git \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# Copy patches
COPY ./patches /patches

RUN git clone ${DRBD_GITREPO} /drbd \
 && cd /drbd \
 && git reset --hard drbd-${DRBD_VERSION} \
 && git config --global user.name "Andrei Kvapil" \
 && git config --global user.email "kvapss@gmail.com" \
 && git am /patches/*.patch \
 && make tarball \
 && mv ./drbd-*.tar.gz /drbd.tar.gz \
 && mv ./docker/entry.sh /entry.sh \
 && chmod +x /entry.sh

FROM $BASE_DEBIAN_BULLSEYE

RUN apt-get update \
 && apt-get install -y kmod gnupg wget make gcc patch curl \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /entry.sh /drbd.tar.gz /

ENV LB_HOW compile
ENTRYPOINT [ "/entry.sh" ]
