ARG BASE_NGINX_ALPINE
FROM $BASE_NGINX_ALPINE

RUN rm -rf /etc/nginx

COPY rootfs /
# To support HTTP PROXY, pass /etc/nginx/madison_http_proxy.tmpl
ENV NGINX_CONFIG_TEMPLATE /etc/nginx/madison.tmpl
CMD envsubst < ${NGINX_CONFIG_TEMPLATE} > /tmp/madison.conf && exec nginx -g 'daemon off;'
