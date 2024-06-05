ARG BASE_NGINX_ALPINE

FROM quay.io/cilium/hubble-ui:v0.12.1@sha256:e15af59a2ded739e420be82de6cbdd0ce22f8d3f00e3a10b3d2d2734e074a394 as artifact

FROM $BASE_NGINX_ALPINE
COPY --from=artifact /etc/nginx/conf.d/default.conf /etc/nginx/conf.d/default.conf
COPY --from=artifact /app /app

ENTRYPOINT ["nginx", "-g", "daemon off;"]
