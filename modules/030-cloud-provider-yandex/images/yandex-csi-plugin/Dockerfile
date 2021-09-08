ARG BASE_ALPINE
FROM registry.deckhouse.io/yandex-csi-driver/yandex-csi-driver:v0.9.11@sha256:4d51f5efb1e90265d048d2d33e07671302b5996b95881ed40e963ca46603eb56 as artifact

FROM $BASE_ALPINE

RUN apk add --no-cache ca-certificates \
                       e2fsprogs \
                       findmnt \
                       xfsprogs \
                       blkid \
                       e2fsprogs-extra

COPY --from=artifact /bin/yandex-csi-driver /bin/yandex-csi-driver

ENTRYPOINT ["/bin/yandex-csi-driver"]
