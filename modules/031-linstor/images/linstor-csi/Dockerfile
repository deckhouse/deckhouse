ARG BASE_DEBIAN
ARG BASE_GOLANG_17_BUSTER

FROM $BASE_GOLANG_17_BUSTER as builder
ARG LINSTOR_CSI_VERSION=0.17.0

RUN git clone https://github.com/linbit/linstor-csi/ /usr/local/go/linstor-csi/ \
 && cd /usr/local/go/linstor-csi \
 && git reset --hard v${LINSTOR_CSI_VERSION} \
 && make -f container.mk staticrelease \
 && mv ./linstor-csi-linux-amd64 /

FROM $BASE_DEBIAN
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      xfsprogs \
      e2fsprogs \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*
COPY --from=builder /linstor-csi-linux-amd64 /usr/bin/linstor-csi
ENTRYPOINT ["/usr/bin/linstor-csi"]
