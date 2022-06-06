# Based on https://github.com/kubernetes/autoscaler/blob/vertical-pod-autoscaler-0.9.0/vertical-pod-autoscaler/pkg/updater/Dockerfile
ARG BASE_ALPINE
FROM registry.k8s.io/autoscaling/vpa-updater:0.9.0@sha256:f2f85053098e957b28e4b4fe8ec60354ce4073ab9a7822c7a2ef18e5a0614a28 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /updater /

ENTRYPOINT ["/updater"]
CMD ["--v=4", "--stderrthreshold=info"]
