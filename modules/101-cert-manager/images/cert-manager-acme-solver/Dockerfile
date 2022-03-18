ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-acmesolver:v1.7.1@sha256:9f39386b03da0dbf9e8208e77a8fbce7f920491c1dc6c4f16d898dbc4c123e74 as artifact
FROM $BASE_ALPINE as final
COPY --from=artifact /app/cmd/acmesolver/acmesolver /bin/acmesolver
RUN apk add --no-cache ca-certificates
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
USER 65534
ENTRYPOINT ["/bin/acmesolver"]
