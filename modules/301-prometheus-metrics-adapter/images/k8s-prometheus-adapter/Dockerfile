ARG BASE_ALPINE
FROM directxman12/k8s-prometheus-adapter-amd64:v0.5.0@sha256:acf4c4dc6aaa51e3004eebdc410b509db45d1a6ebd255457c5745f0b4671678f as artifact

FROM $BASE_ALPINE
COPY --from=artifact /adapter /adapter
ENTRYPOINT [ "/adapter" ]

