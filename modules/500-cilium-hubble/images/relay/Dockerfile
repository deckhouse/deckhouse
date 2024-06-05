ARG BASE_ALPINE
# https://github.com/cilium/cilium/releases/tag/v1.14.3
FROM quay.io/cilium/hubble-relay:v1.14.3@sha256:777c6a94e6c6764eaf5077f03977fd874cc660bf49f34e1642071b87ce614995 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/hubble-relay /usr/local/bin/hubble-relay
RUN chown nobody /usr/local/bin/hubble-relay
RUN chmod +x /usr/local/bin/hubble-relay

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

USER nobody
ENTRYPOINT ["hubble-relay", "serve"]
