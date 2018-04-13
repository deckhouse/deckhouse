FROM alpine:3.6

ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/usr/bin/vts-memory-cleaner"]

RUN apk update && apk add --no-cache bash dumb-init curl jq && rm -rf /var/cache/apk/*

COPY rootfs /
