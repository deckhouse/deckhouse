ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-cainjector:v1.7.1@sha256:985743eeed2b62f68ee06e583f1d5a371e1c35af4b1980a1b2571d29174cce47 as artifact
FROM $BASE_ALPINE as final
COPY --from=artifact /app/cmd/cainjector/cainjector /bin/cainjector
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/cainjector"]
