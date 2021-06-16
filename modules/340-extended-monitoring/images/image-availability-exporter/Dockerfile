ARG BASE_ALPINE
FROM flant/k8s-image-availability-exporter:v0.1.14@sha256:b5338733ead735de7dc0b80588abdbdb591a6ff740d96517ca0fd6ee615fc005 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /k8s-image-availability-exporter /k8s-image-availability-exporter
ENTRYPOINT ["/k8s-image-availability-exporter"]
