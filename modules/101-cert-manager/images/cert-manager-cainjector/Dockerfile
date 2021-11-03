ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-cainjector:v1.5.4@sha256:0c620c74f1b4e9b96195b896b7ea017590cb10cffc712994a169bacd9273c788 as artifact
FROM $BASE_ALPINE as final
COPY --from=artifact /app/cmd/cainjector/cainjector /bin/cainjector
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/cainjector"]
