ARG BASE_DEBIAN_BULLSEYE
ARG BASE_GOLANG_18_BULLSEYE

FROM $BASE_GOLANG_18_BULLSEYE as builder
ARG SPAAS_GITREPO=https://github.com/LINBIT/saas
ARG SPAAS_COMMIT_REF=0824aafa1e1abaedee19c70a98048cea0dedb8ab
ARG DRBD_GITREPO=https://github.com/LINBIT/drbd
ARG DRBD_VERSION=9.1.8

RUN git clone ${SPAAS_GITREPO} /usr/local/go/spaas \
 && cd /usr/local/go/spaas \
 && git reset --hard ${SPAAS_COMMIT_REF} \
 && go build -o /spaas

RUN apt-get update \
 && apt-get install -y make git \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# Using source code from GitHub repository
RUN git clone ${DRBD_GITREPO} /drbd \
 && cd /drbd \
 && git reset --hard drbd-${DRBD_VERSION} \
 && make tarball

# # Using source code provided by LINBIT
# RUN DRBD_PKG=https://pkg.linbit.com//downloads/drbd/9/drbd-${DRBD_VERSION}.tar.gz \
#  && cd /drbd
#  && wget ${DRBD_PKG}

FROM $BASE_DEBIAN_BULLSEYE
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      gcc \
      libc6-dev \
      make \
      coccinelle \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* \
 && ln -sf /proc/mounts /etc/mtab
COPY --from=builder /spaas /
COPY --from=builder /drbd/drbd-*.tar.gz /var/cache/spaas/tarballs/
ENTRYPOINT ["/spaas"]
