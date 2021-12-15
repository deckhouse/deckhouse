ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-cainjector:v1.6.1@sha256:916ef12af73c8a4cbdfb6127d6f513f476f3aeed2447ec7f1a58a95113bda713 as artifact
FROM $BASE_ALPINE as final
COPY --from=artifact /app/cmd/cainjector/cainjector /bin/cainjector
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/cainjector"]
