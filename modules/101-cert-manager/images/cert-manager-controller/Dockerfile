ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-controller:v1.6.1@sha256:fef465f62524ed89c27451752385ab69e5c35ea4bc48b62bf61f733916ea674c as artifact
FROM $BASE_ALPINE as final
COPY --from=artifact /app/cmd/controller/controller /bin/cert-manager-controller
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/cert-manager-controller"]
