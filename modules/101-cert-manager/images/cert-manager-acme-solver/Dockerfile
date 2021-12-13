ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-acmesolver:v1.6.1@sha256:7e4e2e023fcd24b64e597848c8b1bf25674d267dfc8a7b3ad72ce9eb0c75e917 as artifact
FROM $BASE_ALPINE as final
COPY --from=artifact /app/cmd/acmesolver/acmesolver /bin/acmesolver
RUN apk add --no-cache ca-certificates
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
USER 65534
ENTRYPOINT ["/bin/acmesolver"]
