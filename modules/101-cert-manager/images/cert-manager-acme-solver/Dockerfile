ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-acmesolver:v0.10.1@sha256:dca93c9266976f76f68b4c9ad62b21229a12793b162679cef6632a35f52dee6d as artifact
FROM $BASE_ALPINE as final
COPY --from=artifact /app/cmd/acmesolver/acmesolver /bin/acmesolver
RUN apk add --no-cache ca-certificates
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
USER 65534
ENTRYPOINT ["/bin/acmesolver"]
