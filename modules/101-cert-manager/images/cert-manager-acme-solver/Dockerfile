ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-acmesolver:v1.5.4@sha256:364f9a2d5c78c5f7f7f2b901f67e6c0c4d859b9465b5d5abf6f0bbb753031553 as artifact
FROM $BASE_ALPINE as final
COPY --from=artifact /app/cmd/acmesolver/acmesolver /bin/acmesolver
RUN apk add --no-cache ca-certificates
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
USER 65534
ENTRYPOINT ["/bin/acmesolver"]
