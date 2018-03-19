FROM alpine:3.6

ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/usr/bin/iptables-loop"]

RUN apk update && apk add --no-cache bash dumb-init iptables && rm -rf /var/cache/apk/*

COPY rootfs /
