ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-controller:v1.7.1@sha256:51027a4cc4d30e197e3506daf3a4fa2d2a0bc2826469f8a87848dfd279e031c0 as artifact
FROM $BASE_ALPINE as final
COPY --from=artifact /app/cmd/controller/controller /bin/cert-manager-controller
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/cert-manager-controller"]
