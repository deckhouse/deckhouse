ARG BASE_ALPINE
FROM quay.io/cilium/hubble-ui-backend:v0.12.1@sha256:f88a73a120521eeafbcdbd908f517117f6557bf61e115847853fac371f0d774c as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/backend /usr/local/bin/hubble-ui-backend
RUN chown nobody /usr/local/bin/hubble-ui-backend
RUN chmod +x /usr/local/bin/hubble-ui-backend

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

USER nobody
ENTRYPOINT ["hubble-ui-backend"]
