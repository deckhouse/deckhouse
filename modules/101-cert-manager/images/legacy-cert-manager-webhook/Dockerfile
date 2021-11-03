ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-webhook:v0.10.1@sha256:8db898648fe921ce3d4c49a71672d608084b062c03c6295a20d26030ab6077ff as artifact
FROM $BASE_ALPINE
COPY --from=artifact /app/cmd/webhook/webhook /bin/webhook
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/webhook"]
