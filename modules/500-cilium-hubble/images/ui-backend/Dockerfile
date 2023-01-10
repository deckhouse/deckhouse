ARG BASE_ALPINE
FROM quay.io/cilium/hubble-ui-backend:v0.9.2@sha256:a3ac4d5b87889c9f7cc6323e86d3126b0d382933bd64f44382a92778b0cde5d7 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/backend /usr/local/bin/hubble-ui-backend
RUN chown nobody /usr/local/bin/hubble-ui-backend
RUN chmod +x /usr/local/bin/hubble-ui-backend

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

USER nobody
ENTRYPOINT ["hubble-ui-backend"]
