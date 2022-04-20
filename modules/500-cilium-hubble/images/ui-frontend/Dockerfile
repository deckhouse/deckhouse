ARG BASE_NGINX_ALPINE

FROM quay.io/cilium/hubble-ui:v0.8.5@sha256:4eaca1ec1741043cfba6066a165b3bf251590cf4ac66371c4f63fbed2224ebb4 as artifact

FROM $BASE_NGINX_ALPINE
COPY --from=artifact /etc/nginx/conf.d/default.conf /etc/nginx/conf.d/default.conf
COPY --from=artifact /app /app

ENTRYPOINT ["nginx", "-g", "daemon off;"]
