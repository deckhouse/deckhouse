ARG BASE_ALPINE
FROM $BASE_ALPINE

ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/usr/bin/iptables-loop"]

RUN apk update && apk add --no-cache bash dumb-init iptables && rm -rf /var/cache/apk/*

COPY rootfs /
