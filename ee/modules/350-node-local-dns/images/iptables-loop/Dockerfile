ARG BASE_ALPINE
FROM $BASE_ALPINE
COPY iptables-loop.sh /
RUN apk add --no-cache bash iptables inotify-tools

COPY iptables-wrapper-installer.sh /
RUN /iptables-wrapper-installer.sh

ENTRYPOINT ["/iptables-loop.sh"]
