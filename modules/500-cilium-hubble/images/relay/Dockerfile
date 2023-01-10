ARG BASE_ALPINE
# https://github.com/cilium/cilium/releases/tag/v1.11.12
FROM quay.io/cilium/hubble-relay:v1.11.12@sha256:db78c879e19708239c83f028def0fecd6a543abaaf2a58bd17a14d8582628448 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/hubble-relay /usr/local/bin/hubble-relay
RUN chown nobody /usr/local/bin/hubble-relay
RUN chmod +x /usr/local/bin/hubble-relay

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

USER nobody
ENTRYPOINT ["hubble-relay", "serve"]
