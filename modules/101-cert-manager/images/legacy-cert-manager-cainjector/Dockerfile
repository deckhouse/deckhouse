ARG BASE_ALPINE
FROM registry.deckhouse.io/cert-manager/cert-manager-cainjector:v0.10.1@sha256:aaa0d125234ccb2ccab729f7a553dd10c90b9079c25c56263aca80effab6d958 as artifact
FROM $BASE_ALPINE as final
COPY --from=artifact /app/cmd/cainjector/cainjector /bin/cainjector
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/cainjector"]
