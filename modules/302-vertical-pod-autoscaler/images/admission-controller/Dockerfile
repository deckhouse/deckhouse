ARG BASE_ALPINE
FROM registry.k8s.io/autoscaling/vpa-admission-controller:0.12.0@sha256:997b6efac7bda2b0fd9fe8b54feaeac5fb77cadbbe01a46806f3b0c84a354429 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /admission-controller /

ENTRYPOINT ["/admission-controller"]
CMD ["--v=4", "--stderrthreshold=info"]
