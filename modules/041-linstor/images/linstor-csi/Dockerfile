ARG BASE_UBUNTU
ARG BASE_GOLANG_18_BULLSEYE

FROM $BASE_GOLANG_18_BULLSEYE as builder
ARG LINSTOR_CSI_GITREPO=https://github.com/linbit/linstor-csi
ARG LINSTOR_CSI_VERSION=0.22.1
ARG LINSTOR_WAIT_UNTIL_GITREPO=https://github.com/LINBIT/linstor-wait-until
ARG LINSTOR_WAIT_UNTIL_VERSION=0.2.1

RUN git clone ${LINSTOR_CSI_GITREPO} /usr/local/go/linstor-csi/ \
 && cd /usr/local/go/linstor-csi \
 && git reset --hard v${LINSTOR_CSI_VERSION} \
 && cd cmd/linstor-csi \
 && go build -ldflags="-X github.com/piraeusdatastore/linstor-csi/pkg/driver.Version=v${LINSTOR_CSI_VERSION}" \
 && mv ./linstor-csi /

RUN git clone ${LINSTOR_WAIT_UNTIL_GITREPO} /usr/local/go/linstor-wait-until \
 && cd /usr/local/go/linstor-wait-until \
 && git reset --hard v${LINSTOR_WAIT_UNTIL_VERSION} \
 && go build \
 && mv ./linstor-wait-until /

FROM $BASE_UBUNTU
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      xfsprogs \
      e2fsprogs \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* \
 && ln -sf /proc/mounts /etc/mtab

COPY --from=builder /linstor-csi /linstor-wait-until /
ENTRYPOINT ["/linstor-csi"]
