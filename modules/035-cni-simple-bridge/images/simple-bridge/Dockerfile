ARG BASE_ALPINE
FROM $BASE_ALPINE

RUN apk add --no-cache curl jq bash grep iptables iproute2 python3

ADD rootfs /
COPY iptables-wrapper-installer.sh /
RUN /iptables-wrapper-installer.sh
