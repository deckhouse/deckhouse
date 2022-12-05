ARG BASE_ALPINE
FROM quay.io/cilium/hubble-ui-backend:v0.8.5@sha256:2bce50cf6c32719d072706f7ceccad654bfa907b2745a496da99610776fe31ed as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/backend /usr/local/bin/hubble-ui-backend
RUN chown nobody /usr/local/bin/hubble-ui-backend
RUN chmod +x /usr/local/bin/hubble-ui-backend

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

USER nobody
ENTRYPOINT ["hubble-ui-backend"]
