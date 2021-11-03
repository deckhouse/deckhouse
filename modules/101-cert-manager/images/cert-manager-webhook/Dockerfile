ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-webhook:v1.5.4@sha256:2bcb5e52a82ba61926883e2076cf7977e9b7affe39f3c9a2022c22a37c0fa5b8 as artifact
FROM $BASE_ALPINE
COPY --from=artifact /app/cmd/webhook/webhook /bin/webhook
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/webhook"]
