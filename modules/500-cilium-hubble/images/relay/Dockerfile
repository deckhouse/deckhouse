ARG BASE_ALPINE
# https://github.com/cilium/cilium/releases/tag/v1.14.3
FROM quay.io/cilium/hubble-relay:v1.14.5@sha256:faf8e9c24b94fd5c4e0e52bd72bae30913dd92cd2271bf4db5c974e56e405a1d as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/hubble-relay /usr/local/bin/hubble-relay
RUN chown nobody /usr/local/bin/hubble-relay
RUN chmod +x /usr/local/bin/hubble-relay

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

USER nobody
ENTRYPOINT ["hubble-relay", "serve"]
