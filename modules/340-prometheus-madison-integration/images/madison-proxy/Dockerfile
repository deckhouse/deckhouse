FROM nginx:1.15-alpine

RUN rm -rf /etc/nginx

COPY rootfs /

CMD envsubst < /etc/nginx/nginx.tmpl > /etc/nginx/nginx.conf && exec nginx -g 'daemon off;'
