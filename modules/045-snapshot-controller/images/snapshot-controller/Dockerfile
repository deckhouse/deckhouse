# Based on https://github.com/kubernetes-csi/external-snapshotter/blob/master/cmd/snapshot-controller/Dockerfile
ARG BASE_ALPINE
FROM registry.k8s.io/sig-storage/snapshot-controller:v5.0.1@sha256:cc9f25f394a50acd54df580458e0470b1b804dfc8ada59924d51667da9efb165 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /snapshot-controller /snapshot-controller
ENTRYPOINT [ "/snapshot-controller" ]

