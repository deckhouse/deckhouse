ARG BASE_ALPINE
FROM flant/yandex-csi-plugin:v0.9.10@sha256:663398eec1bae800c028d62f7657e62358ed350e4f98a7e48d1124314a9cc10b as artifact

FROM $BASE_ALPINE

RUN apk add --no-cache ca-certificates \
                       e2fsprogs \
                       findmnt \
                       xfsprogs \
                       blkid \
                       e2fsprogs-extra

COPY --from=artifact /bin/yandex-csi-driver /bin/yandex-csi-driver

ENTRYPOINT ["/bin/yandex-csi-driver"]
