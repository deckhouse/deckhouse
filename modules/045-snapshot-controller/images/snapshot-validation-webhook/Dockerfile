# Based on https://github.com/kubernetes-csi/external-snapshotter/blob/master/cmd/snapshot-validation-webhook/Dockerfile
ARG BASE_ALPINE
FROM registry.k8s.io/sig-storage/snapshot-validation-webhook:v5.0.1@sha256:78b3784b6fcb96b9f1806040fbb55f72bf55c5a1f599db451ef9ff3c7b282310 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /snapshot-validation-webhook /snapshot-validation-webhook
ENTRYPOINT [ "/snapshot-validation-webhook" ]

