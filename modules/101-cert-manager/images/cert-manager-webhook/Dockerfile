ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-webhook:v1.6.1@sha256:45934ab42749e8c90da0726734155374f4ea55d7796246264e7adea87569918a as artifact
FROM $BASE_ALPINE
COPY --from=artifact /app/cmd/webhook/webhook /bin/webhook
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/webhook"]
