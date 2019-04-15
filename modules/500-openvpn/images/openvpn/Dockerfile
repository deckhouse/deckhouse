FROM python:3.6-alpine
RUN apk update && \
    apk add bash iptables bind-tools netcat-openbsd && \
    apk add openssl easy-rsa openvpn && \
    mkdir -p /dev/net && \
    mknod /dev/net/tun c 10 200 && \
    rm -rf /tmp/* /var/tmp/* /var/cache/apk/* /var/cache/distfiles/* || exit 0
