ARG BASE_NGINX_ALPINE
FROM $BASE_NGINX_ALPINE

RUN rm -rf /etc/nginx

COPY rootfs /

CMD envsubst < /etc/nginx/nginx.tmpl > /etc/nginx/nginx.conf && exec nginx -g 'daemon off;'
