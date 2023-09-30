ARG BASE_ALPINE
FROM registry.deckhouse.io/yandex-csi-driver/yandex-csi-driver:v0.10.0@sha256:5bbefd73169b25bb17723515569fea8ed4423c9e4ba87f8d348afa4f323567fc as artifact

FROM $BASE_ALPINE

RUN apk add --no-cache ca-certificates \
                       e2fsprogs \
                       findmnt \
                       xfsprogs \
                       blkid \
                       e2fsprogs-extra

COPY --from=artifact /bin/yandex-csi-driver /bin/yandex-csi-driver

ENTRYPOINT ["/bin/yandex-csi-driver"]
