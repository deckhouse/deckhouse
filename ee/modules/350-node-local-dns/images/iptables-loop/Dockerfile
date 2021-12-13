ARG BASE_ALPINE
FROM $BASE_ALPINE
COPY iptables-loop.sh /
RUN apk add --no-cache bash iptables inotify-tools
ENTRYPOINT ["/iptables-loop.sh"]
