ARG BASE_PYTHON_ALPINE
FROM $BASE_PYTHON_ALPINE
RUN apk update && \
    apk add bash iptables bind-tools netcat-openbsd curl && \
    apk add openssl openvpn && \
    mkdir /usr/share/easy-rsa && \
    cd /usr/share/easy-rsa && \
    curl -sL https://github.com/OpenVPN/easy-rsa/releases/download/v3.0.5/EasyRSA-nix-3.0.5.tgz | tar -xzv --strip-components=1 -C . && \
    rm -rf /tmp/* /var/tmp/* /var/cache/apk/* /var/cache/distfiles/* || exit 0
