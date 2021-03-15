ARG BASE_NGINX_ALPINE

FROM $BASE_NGINX_ALPINE
RUN rm -rf /etc/nginx

COPY rootfs /
COPY entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
