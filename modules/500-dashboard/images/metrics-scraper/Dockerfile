# Based on https://github.com/kubernetes-sigs/dashboard-metrics-scraper/blob/v1.0.6/Dockerfile
ARG BASE_ALPINE
FROM kubernetesui/metrics-scraper:v1.0.6@sha256:1f977343873ed0e2efd4916a6b2f3075f310ff6fe42ee098f54fc58aa7a28ab7 as artifact

FROM $BASE_ALPINE

COPY --from=artifact /etc/passwd /etc/passwd
COPY --from=artifact /metrics-sidecar /metrics-sidecar

USER nonroot
EXPOSE 8080

ENTRYPOINT ["/metrics-sidecar"]
