ARG BASE_ALPINE
FROM registry.deckhouse.io/yandex-csi-driver/yandex-csi-driver:v0.9.12@sha256:5aba630c3e7c8b7acfbfb1f8ded22f5dabd803dcdef34a21ecc04144fcec0630 as artifact

FROM $BASE_ALPINE

RUN apk add --no-cache ca-certificates \
                       e2fsprogs \
                       findmnt \
                       xfsprogs \
                       blkid \
                       e2fsprogs-extra

COPY --from=artifact /bin/yandex-csi-driver /bin/yandex-csi-driver

ENTRYPOINT ["/bin/yandex-csi-driver"]
