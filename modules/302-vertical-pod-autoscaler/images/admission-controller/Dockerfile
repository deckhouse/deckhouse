# Based on https://github.com/kubernetes/autoscaler/blob/vertical-pod-autoscaler-0.9.0/vertical-pod-autoscaler/pkg/admission-controller/Dockerfile
ARG BASE_ALPINE
FROM registry.k8s.io/autoscaling/vpa-admission-controller:0.9.0@sha256:690e8d48fb6e11fac2be2cfe3f699bd864ae5e961e35b8dd292102104ecae521 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /admission-controller /

ENTRYPOINT ["/admission-controller"]
CMD ["--v=4", "--stderrthreshold=info"]
