ARG BASE_NGINX_ALPINE
FROM $BASE_NGINX_ALPINE

RUN rm -rf /etc/nginx

COPY rootfs /

CMD envsubst < /etc/nginx/madison.tmpl > /tmp/madison.conf && exec nginx -g 'daemon off;'
