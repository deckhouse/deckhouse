ARG BASE_ALPINE
FROM quay.io/cilium/hubble-relay:v1.11.4@sha256:9e56ba4bec014a81f9da4ea758fb2e73461263dea407851224429e500663db3b as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/hubble-relay /usr/local/bin/hubble-relay
RUN chown nobody /usr/local/bin/hubble-relay
RUN chmod +x /usr/local/bin/hubble-relay

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

USER nobody
ENTRYPOINT ["hubble-relay", "serve"]
