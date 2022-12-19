ARG BASE_ALPINE
FROM quay.io/cilium/hubble-relay:v1.12.5@sha256:dc5b396e94f986f83ccef89f13a91c29df482d4af491ff3bd4d40c05873d351a as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/hubble-relay /usr/local/bin/hubble-relay
RUN chown nobody /usr/local/bin/hubble-relay
RUN chmod +x /usr/local/bin/hubble-relay

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

USER nobody
ENTRYPOINT ["hubble-relay", "serve"]
