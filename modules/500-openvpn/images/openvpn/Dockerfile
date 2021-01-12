ARG BASE_PYTHON_ALPINE
FROM $BASE_PYTHON_ALPINE
RUN apk update && \
    apk add bash iptables bind-tools netcat-openbsd && \
    apk add openssl easy-rsa openvpn && \
    rm -rf /tmp/* /var/tmp/* /var/cache/apk/* /var/cache/distfiles/* || exit 0
