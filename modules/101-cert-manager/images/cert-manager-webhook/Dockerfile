ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-webhook:v1.7.1@sha256:a926d60b6f23553ca5d11ac9cd66bcc692136e838613c8bc0d60c6c35a3cbcfc as artifact
FROM $BASE_ALPINE
COPY --from=artifact /app/cmd/webhook/webhook /bin/webhook
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/webhook"]
