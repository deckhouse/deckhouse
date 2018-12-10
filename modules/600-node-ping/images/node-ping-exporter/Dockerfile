FROM alpine

RUN apk add --no-cache iputils jq bc bash dumb-init

COPY rootfs /

ENTRYPOINT ["/usr/bin/dumb-init", "--", "node-ping-runner"]
