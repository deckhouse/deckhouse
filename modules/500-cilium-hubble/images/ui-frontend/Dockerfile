ARG BASE_NGINX_ALPINE

FROM quay.io/cilium/hubble-ui:v0.9.2@sha256:d3596efc94a41c6b772b9afe6fe47c17417658956e04c3e2a28d293f2670663e as artifact

FROM $BASE_NGINX_ALPINE
COPY --from=artifact /etc/nginx/conf.d/default.conf /etc/nginx/conf.d/default.conf
COPY --from=artifact /app /app

ENTRYPOINT ["nginx", "-g", "daemon off;"]
