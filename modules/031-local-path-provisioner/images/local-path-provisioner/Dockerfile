ARG BASE_ALPINE
FROM rancher/local-path-provisioner:v0.0.20@sha256:d5999b20a1b180940061677db3bdb48dd7eb432cd48147c4ff15469fb74ade80 as artifact

FROM $BASE_ALPINE

RUN apk add --no-cache ca-certificates \
                       e2fsprogs \
                       findmnt \
                       xfsprogs \
                       blkid \
                       e2fsprogs-extra

COPY --from=artifact /usr/bin/local-path-provisioner /usr/bin/local-path-provisioner

ENTRYPOINT ["/usr/bin/local-path-provisioner"]
