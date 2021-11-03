ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-controller:v1.5.4@sha256:ed41b2cd693c2716b3910619746a22ec09a9d4134b82b05834b5f51680301b96 as artifact
FROM $BASE_ALPINE as final
COPY --from=artifact /app/cmd/controller/controller /bin/cert-manager-controller
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/cert-manager-controller"]
